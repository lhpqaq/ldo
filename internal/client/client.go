package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Client struct {
	baseURL   string
	client    tls_client.HttpClient
	jar       http.CookieJar
	csrfToken string
	username  string
	headers   http.Header
}

type TopicList struct {
	Users     []User `json:"users"`
	TopicList struct {
		Topics        []Topic `json:"topics"`
		MoreTopicsURL string  `json:"more_topics_url"`
	} `json:"topic_list"`
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type Topic struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	ReplyCount   int    `json:"reply_count"`
	PostsCount   int    `json:"posts_count"`
	Views        int    `json:"views"`
	CategoryID   int    `json:"category_id"`
	Pinned       bool   `json:"pinned"`
	Visible      bool   `json:"visible"`
	Closed       bool   `json:"closed"`
	Archived     bool   `json:"archived"`
	LastPostedAt string `json:"last_posted_at"`
}

type TopicDetail struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	CategoryID int    `json:"category_id"`
	PostsCount int    `json:"posts_count"`
	PostStream struct {
		Posts  []Post `json:"posts"`
		Stream []int  `json:"stream"` // 所有帖子ID列表
	} `json:"post_stream"`
}

type Post struct {
	ID             int             `json:"id"`
	Username       string          `json:"username"`
	Raw            string          `json:"raw"`
	Cooked         string          `json:"cooked"`
	PostNumber     int             `json:"post_number"`
	CreatedAt      string          `json:"created_at"`
	ActionsSummary []ActionSummary `json:"actions_summary"`
}

type ActionSummary struct {
	ID    int  `json:"id"`
	Acted bool `json:"acted"`
}

type savedCookies struct {
	Cookies  []*http.Cookie `json:"cookies"`
	Username string         `json:"username"`
	SavedAt  time.Time      `json:"saved_at"`
}

func NewClient(baseURL, username, password string) (*Client, error) {
	jar := tls_client.NewCookieJar()

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_124),
		tls_client.WithCookieJar(jar),
		tls_client.WithRandomTLSExtensionOrder(),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, err
	}

	commonHeaders := http.Header{
		"sec-ch-ua":                 {`"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"Windows"`},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36`},
		"accept":                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8`},
		"accept-language":           {`zh-CN,zh;q=0.9`},
	}

	c := &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   client,
		jar:      jar,
		headers:  commonHeaders,
		username: username,
	}

	if err := c.loadCookies(); err == nil {
		if c.verifyCookies() {
			fmt.Println("✅ 使用已保存的登录状态")
			return c, nil
		}
		fmt.Println("⚠️  已保存的登录状态已失效，重新登录...")
	}

	if err := c.warmup(); err != nil {
		return nil, fmt.Errorf("预热失败: %w", err)
	}

	if err := c.login(username, password); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}

	if err := c.saveCookies(); err != nil {
		fmt.Printf("⚠️  保存 Cookie 失败: %v\n", err)
	} else {
		fmt.Println("✅ 登录状态已保存")
	}

	return c, nil
}

func (c *Client) getCookieFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".linuxdo_cookies.json")
}

func (c *Client) saveCookies() error {
	u, _ := url.Parse(c.baseURL)
	cookies := c.jar.Cookies(u)

	saved := savedCookies{
		Cookies:  cookies,
		Username: c.username,
		SavedAt:  time.Now(),
	}

	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}

	return os.WriteFile(c.getCookieFilePath(), data, 0600)
}

func (c *Client) loadCookies() error {
	data, err := os.ReadFile(c.getCookieFilePath())
	if err != nil {
		return err
	}

	var saved savedCookies
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	if saved.Username != c.username {
		return fmt.Errorf("用户名不匹配")
	}

	if time.Since(saved.SavedAt) > 7*24*time.Hour {
		return fmt.Errorf("cookie 已过期")
	}

	u, _ := url.Parse(c.baseURL)
	c.jar.SetCookies(u, saved.Cookies)

	return c.fetchCSRF()
}

func (c *Client) verifyCookies() bool {
	_, err := c.GetLatestTopics()
	return err == nil
}

func (c *Client) warmup() error {
	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/", nil)
	req.Header = c.headers.Clone()

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	time.Sleep(2 * time.Second)
	return nil
}

func (c *Client) fetchCSRF() error {
	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/session/csrf", nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.baseURL+"/login")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var csrfData struct {
		Csrf string `json:"csrf"`
	}
	if err := json.Unmarshal(bodyBytes, &csrfData); err != nil {
		return err
	}

	c.csrfToken = csrfData.Csrf
	return nil
}

func (c *Client) login(username, password string) error {
	if err := c.fetchCSRF(); err != nil {
		return err
	}

	formData := url.Values{}
	formData.Set("login", username)
	formData.Set("password", password)
	formData.Set("second_factor_method", "1")
	formData.Set("timezone", "Asia/Shanghai")

	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/session", strings.NewReader(formData.Encode()))
	req.Header = c.headers.Clone()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/login")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var loginResult map[string]any
	json.Unmarshal(bodyBytes, &loginResult)
	if errStr, ok := loginResult["error"]; ok {
		return fmt.Errorf("登录错误: %v", errStr)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("登录失败，状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) GetLatestTopics() (*TopicList, error) {
	return c.getTopics("/latest.json")
}

func (c *Client) GetHotTopics() (*TopicList, error) {
	return c.getTopics("/hot.json")
}

func (c *Client) GetNewTopics() (*TopicList, error) {
	return c.getTopics("/new.json")
}

func (c *Client) GetTopTopics(period string) (*TopicList, error) {
	return c.getTopics("/top.json?period=" + period)
}

func (c *Client) getTopics(path string) (*TopicList, error) {
	req, _ := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var topicList TopicList
	if err := json.Unmarshal(bodyBytes, &topicList); err != nil {
		return nil, err
	}

	return &topicList, nil
}

func (c *Client) GetTopic(id int) (*TopicDetail, error) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/t/%d.json", c.baseURL, id), nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var detail TopicDetail
	if err := json.Unmarshal(bodyBytes, &detail); err != nil {
		return nil, err
	}

	return &detail, nil
}

// GetPostsByIDs 根据帖子ID列表获取帖子内容
func (c *Client) GetPostsByIDs(topicID int, postIDs []int) ([]Post, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}

	// 构建帖子ID字符串
	var postIDStrs []string
	for _, id := range postIDs {
		postIDStrs = append(postIDStrs, fmt.Sprintf("%d", id))
	}
	postIDsParam := strings.Join(postIDStrs, ",")

	url := fmt.Sprintf("%s/t/%d/posts.json?post_ids[]=%s", c.baseURL, topicID, postIDsParam)
	
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		PostStream struct {
			Posts []Post `json:"posts"`
		} `json:"post_stream"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	return result.PostStream.Posts, nil
}

func (c *Client) CreatePost(topicID int, raw string, replyToPostNumber int) error {
	payload := map[string]any{
		"topic_id": topicID,
		"raw":      raw,
	}
	if replyToPostNumber > 0 {
		payload["reply_to_post_number"] = replyToPostNumber
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/posts.json", strings.NewReader(string(jsonData)))
	req.Header = c.headers.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", fmt.Sprintf("%s/t/%d", c.baseURL, topicID))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("发帖失败 (状态码 %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) LikePost(postID int) error {
	payload := map[string]any{
		"id":                   postID,
		"post_action_type_id": 2,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/post_actions.json", strings.NewReader(string(jsonData)))
	req.Header = c.headers.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	return nil
}

func (c *Client) UnlikePost(postID int) error {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/post_actions/%d.json?post_action_type_id=2", c.baseURL, postID), nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	return nil
}

func (c *Client) GetUsername() string {
	return c.username
}

func (c *Client) GetMoreTopics(moreURL string) (*TopicList, error) {
	fullURL := c.baseURL + moreURL
	
	req, _ := http.NewRequest(http.MethodGet, fullURL, nil)
	req.Header = c.headers.Clone()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("被 Cloudflare 拦截 (403)")
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var topicList TopicList
	if err := json.Unmarshal(bodyBytes, &topicList); err != nil {
		return nil, err
	}

	return &topicList, nil
}
