package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lhpqaq/ldo/internal/client"
)

const (
	stateFile       = ".lottery_agent_state.json"
	checkInterval   = 5 * time.Minute // 每5分钟检查一次
	maxTopicsCheck  = 200             // 每次检查前200个话题
	maxPages        = 4               // 最多加载4页（每页约50条）
	preloadHistory  = true            // 是否预加载历史回复记录
)

var (
	// 抽奖关键词
	lotteryKeywords = []string{
		"抽奖",
		"抽取",
	}

	// 回复话术
	replies = []string{
		"参与一下",
		"谢谢大佬",
		"参与参与",
		"感谢分享",
		"来了来了",
		"支持支持",
		"来参与一下",
		"参与一下，万一中了呢",
		"来了来了来了",
	}
)

type AgentState struct {
	RepliedTopics map[int]time.Time `json:"replied_topics"` // 已回复的话题ID -> 回复时间
	LastCheck     time.Time         `json:"last_check"`     // 上次检查时间
}

type LotteryAgent struct {
	client *client.Client
	state  *AgentState
}

func main() {
	username := os.Getenv("LINUXDO_USERNAME")
	password := os.Getenv("LINUXDO_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("请设置 LINUXDO_USERNAME 和 LINUXDO_PASSWORD 环境变量")
	}

	fmt.Println("🤖 Linux.do 抽奖助手启动中...")

	c, err := client.NewClient("https://linux.do", username, password)
	if err != nil {
		log.Fatalf("客户端初始化失败: %v", err)
	}

	fmt.Printf("✅ 登录成功! 用户: %s\n", c.GetUsername())

	agent := &LotteryAgent{
		client: c,
		state:  loadState(),
	}

	// 预加载历史回复记录
	if preloadHistory {
		agent.preloadRepliedTopics()
	}

	// 清理30天前的记录
	agent.cleanOldRecords()

	fmt.Println("🔍 开始监控抽奖帖...")
	fmt.Printf("⏰ 检查间隔: %v\n", checkInterval)
	fmt.Println("💬 回复话术:", replies)
	fmt.Println()

	// 首次检查
	agent.checkAndReply()

	// 定时检查
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		agent.checkAndReply()
	}
}

// preloadRepliedTopics 从服务器预加载用户已回复过的话题
func (a *LotteryAgent) preloadRepliedTopics() {
	fmt.Println("📥 正在从服务器加载历史回复记录...")

	repliedTopics, err := a.client.GetUserRepliedTopics()
	if err != nil {
		log.Printf("⚠️  加载历史回复失败: %v，继续使用本地记录\n", err)
		return
	}

	// 合并到本地状态
	newCount := 0
	for topicID := range repliedTopics {
		if _, exists := a.state.RepliedTopics[topicID]; !exists {
			a.state.RepliedTopics[topicID] = time.Now()
			newCount++
		}
	}

	if newCount > 0 {
		fmt.Printf("✅ 从服务器加载了 %d 条新的回复记录\n", newCount)
		a.saveState()
	} else {
		fmt.Println("✅ 本地记录已是最新")
	}

	fmt.Printf("📊 总计已回复话题: %d 个\n", len(a.state.RepliedTopics))
}

func (a *LotteryAgent) checkAndReply() {
	fmt.Printf("\n[%s] 开始检查新帖...\n", time.Now().Format("2006-01-02 15:04:05"))

	// 优先使用未读话题接口
	topics, err := a.client.GetUnreadTopics()
	if err != nil {
		log.Printf("⚠️  获取未读话题失败，回退到最新话题: %v\n", err)
		topics, err = a.client.GetLatestTopics()
		if err != nil {
			log.Printf("❌ 获取话题失败: %v\n", err)
			return
		}
	}

	checked := 0
	found := 0
	replied := 0
	page := 1
	moreURL := topics.TopicList.MoreTopicsURL

	// 收集所有要检查的话题
	allTopics := topics.TopicList.Topics

	// 自动加载更多页，直到达到限制
	for page < maxPages && moreURL != "" && len(allTopics) < maxTopicsCheck {
		fmt.Printf("📄 加载第 %d 页...\n", page+1)
		moreTopics, err := a.client.GetMoreTopics(moreURL)
		if err != nil {
			log.Printf("⚠️  加载更多话题失败: %v\n", err)
			break
		}
		allTopics = append(allTopics, moreTopics.TopicList.Topics...)
		moreURL = moreTopics.TopicList.MoreTopicsURL
		page++
		time.Sleep(1 * time.Second) // 避免请求过快
	}

	fmt.Printf("📚 共加载 %d 个话题，开始检查...\n", len(allTopics))

	for _, topic := range allTopics {
		if checked >= maxTopicsCheck {
			break
		}
		checked++

		// 先检查本地记录，避免重复API调用
		if _, exists := a.state.RepliedTopics[topic.ID]; exists {
			continue
		}

		// 检查标题是否包含关键词
		titleMatch := containsLotteryKeyword(topic.Title)

		// 获取话题详情，检查第一楼内容
		detail, err := a.client.GetTopic(topic.ID)
		if err != nil {
			log.Printf("   ⚠️  获取话题 [%d] 详情失败: %v\n", topic.ID, err)
			continue
		}

		// 检查第一楼内容是否包含关键词
		contentMatch := false
		if len(detail.PostStream.Posts) > 0 {
			firstPost := detail.PostStream.Posts[0]
			// 同时检查 Raw 和 Cooked 字段
			contentMatch = containsLotteryKeyword(firstPost.Raw) ||
				containsLotteryKeyword(firstPost.Cooked)
		}

		// 标题和内容都不匹配，跳过
		if !titleMatch && !contentMatch {
			continue
		}

		found++
		fmt.Printf("🎉 发现抽奖帖: [%d] %s (标题:%v 内容:%v)\n",
			topic.ID, topic.Title, titleMatch, contentMatch)

		// 检查是否已经回复过（服务器验证）
		if a.hasReplied(detail) {
			fmt.Printf("   ℹ️  已经回复过此帖，跳过\n")
			a.state.RepliedTopics[topic.ID] = time.Now()
			a.saveState()
			continue
		}

		// 随机选择一个回复
		reply := replies[rand.Intn(len(replies))]

		// 等待随机时间（1-5秒），看起来更自然
		waitTime := time.Duration(1+rand.Intn(4)) * time.Second
		time.Sleep(waitTime)

		// 发送回复
		err = a.client.CreatePost(topic.ID, reply, 0)
		if err != nil {
			log.Printf("   ❌ 回复失败: %v\n", err)
			continue
		}

		replied++
		fmt.Printf("   ✅ 已回复: \"%s\"\n", reply)

		// 记录已回复
		a.state.RepliedTopics[topic.ID] = time.Now()
		a.saveState()

		// 避免频繁操作，等待一段时间
		if replied < 3 {
			time.Sleep(time.Duration(5+rand.Intn(10)) * time.Second)
		}
	}

	a.state.LastCheck = time.Now()
	a.saveState()

	fmt.Printf("📊 检查完成: 加载了 %d 页，检查了 %d 个话题, 发现 %d 个抽奖帖, 新回复 %d 个\n", page, checked, found, replied)
}

// hasReplied 检查当前用户是否已在该话题中回复过
// 判断逻辑：遍历话题中的所有帖子，检查是否有当前用户发布的回复（排除第一楼）
func (a *LotteryAgent) hasReplied(detail *client.TopicDetail) bool {
	username := a.client.GetUsername()

	// 检查已加载的帖子
	for _, post := range detail.PostStream.Posts {
		// post_number > 1 表示这是回复，不是主题帖
		if post.Username == username && post.PostNumber > 1 {
			return true
		}
	}

	// 如果话题有很多回复，可能没有全部加载
	// 这里我们相对保守：如果已加载的帖子中没找到，就认为没回复过
	// 因为我们主要关注新帖，通常回复不会太多
	return false
}

func containsLotteryKeyword(text string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range lotteryKeywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (a *LotteryAgent) cleanOldRecords() {
	threshold := time.Now().AddDate(0, 0, -30) // 30天前
	count := 0

	for topicID, replyTime := range a.state.RepliedTopics {
		if replyTime.Before(threshold) {
			delete(a.state.RepliedTopics, topicID)
			count++
		}
	}

	if count > 0 {
		fmt.Printf("🧹 清理了 %d 条30天前的记录\n", count)
		a.saveState()
	}
}

func loadState() *AgentState {
	homeDir, _ := os.UserHomeDir()
	stateFilePath := filepath.Join(homeDir, stateFile)

	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		// 文件不存在，返回新状态
		return &AgentState{
			RepliedTopics: make(map[int]time.Time),
		}
	}

	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("⚠️  读取状态文件失败，使用新状态: %v\n", err)
		return &AgentState{
			RepliedTopics: make(map[int]time.Time),
		}
	}

	if state.RepliedTopics == nil {
		state.RepliedTopics = make(map[int]time.Time)
	}

	fmt.Printf("📂 加载本地状态: 已记录 %d 个已回复话题\n", len(state.RepliedTopics))
	return &state
}

func (a *LotteryAgent) saveState() {
	homeDir, _ := os.UserHomeDir()
	stateFilePath := filepath.Join(homeDir, stateFile)

	data, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		log.Printf("⚠️  序列化状态失败: %v\n", err)
		return
	}

	if err := os.WriteFile(stateFilePath, data, 0600); err != nil {
		log.Printf("⚠️  保存状态失败: %v\n", err)
	}
}
