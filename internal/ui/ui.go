package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lhpqaq/ldo/internal/client"
)

type viewState int

const (
	topicListView viewState = iota
	topicDetailView
	composerView
	jumpInputView
)

type Model struct {
	client        *client.Client
	state         viewState
	topics        []client.Topic
	users         map[int]string
	selected      int
	topicDetail   *client.TopicDetail
	posts         []client.Post
	allPostIDs    []int  // 所有帖子的ID
	currentPostIdx int   // 当前显示的帖子索引
	viewport      viewport.Model
	composer      textarea.Model
	jumpInput     textarea.Model
	filter        string
	err           error
	width         int
	height        int
	ready         bool
	replyToPost   int
	moreTopicsURL string
	loading       bool
}

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Filter   key.Binding
	Reply    key.Binding
	Like     key.Binding
	Refresh  key.Binding
	Open     key.Binding
	LoadMore key.Binding
	Jump     key.Binding
	Last     key.Binding
}

var keys = keyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k")),
	Down:     key.NewBinding(key.WithKeys("down", "j")),
	Enter:    key.NewBinding(key.WithKeys("enter")),
	Back:     key.NewBinding(key.WithKeys("esc")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c")),
	Filter:   key.NewBinding(key.WithKeys("f")),
	Reply:    key.NewBinding(key.WithKeys("r")),
	Like:     key.NewBinding(key.WithKeys("l")),
	Refresh:  key.NewBinding(key.WithKeys("g")),
	Open:     key.NewBinding(key.WithKeys("o")),
	LoadMore: key.NewBinding(key.WithKeys("n")),
	Jump:     key.NewBinding(key.WithKeys("/")),
	Last:     key.NewBinding(key.WithKeys("G")),
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(1).
			PaddingRight(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
	
	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)
)

func NewModel(c *client.Client) Model {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.CharLimit = 0
	ta.SetWidth(100)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false
	ta.Focus()

	jumpTA := textarea.New()
	jumpTA.Placeholder = ""
	jumpTA.CharLimit = 10
	jumpTA.SetWidth(30)
	jumpTA.SetHeight(1)
	jumpTA.ShowLineNumbers = false

	vp := viewport.New(0, 0)

	return Model{
		client:    c,
		state:     topicListView,
		filter:    "latest",
		composer:  ta,
		jumpInput: jumpTA,
		viewport:  vp,
		users:     make(map[int]string),
		loading:   false,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchTopics
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		m.composer.SetWidth(msg.Width - 8)
		m.composer.SetHeight(msg.Height - 15)
		m.ready = true

	case tea.KeyMsg:
		switch m.state {
		case topicListView:
			return m.updateTopicList(msg)
		case topicDetailView:
			return m.updateTopicDetail(msg)
		case composerView:
			return m.updateComposer(msg)
		case jumpInputView:
			return m.updateJumpInput(msg)
		}

	case topicListMsg:
		m.loading = false
		if msg.append {
			m.topics = append(m.topics, msg.topics...)
			for k, v := range msg.users {
				m.users[k] = v
			}
		} else {
			m.topics = msg.topics
			m.users = msg.users
		}
		m.moreTopicsURL = msg.moreURL
		m.err = msg.err

	case topicDetailMsg:
		m.topicDetail = msg.detail
		m.posts = msg.posts
		m.allPostIDs = msg.allPostIDs
		m.currentPostIdx = 0
		m.err = msg.err
		if msg.detail != nil {
			m.viewport.SetContent(m.renderTopicDetail())
		}

	case morePostsMsg:
		if msg.err == nil && len(msg.posts) > 0 {
			m.posts = append(m.posts, msg.posts...)
			m.currentPostIdx = len(m.posts) - 1
			m.viewport.SetContent(m.renderTopicDetail())
		}
		m.err = msg.err

	case jumpToPostMsg:
		if msg.err == nil && len(msg.posts) > 0 {
			m.posts = msg.posts
			m.currentPostIdx = msg.targetIdx
			m.viewport.SetContent(m.renderTopicDetail())
		}
		m.err = msg.err

	case postCreatedMsg:
		if msg.err == nil {
			m.state = topicDetailView
			m.composer.Reset()
			return m, m.fetchTopicDetail(m.topicDetail.ID)
		}
		m.err = msg.err
	}

	if m.state == composerView {
		m.composer, cmd = m.composer.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.state == topicDetailView {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.state == jumpInputView {
		m.jumpInput, cmd = m.jumpInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateTopicList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		if m.selected > 0 {
			m.selected--
		}
	case key.Matches(msg, keys.Down):
		if m.selected < len(m.topics)-1 {
			m.selected++
		}
		if m.selected >= len(m.topics)-5 && m.moreTopicsURL != "" && !m.loading {
			m.loading = true
			return m, m.loadMoreTopics
		}
	case key.Matches(msg, keys.Enter):
		if len(m.topics) > 0 {
			m.state = topicDetailView
			return m, m.fetchTopicDetail(m.topics[m.selected].ID)
		}
	case key.Matches(msg, keys.Open):
		if len(m.topics) > 0 {
			topicID := m.topics[m.selected].ID
			openInBrowser(fmt.Sprintf("https://linux.do/t/%d", topicID))
		}
	case key.Matches(msg, keys.LoadMore):
		if m.moreTopicsURL != "" && !m.loading {
			m.loading = true
			return m, m.loadMoreTopics
		}
	case key.Matches(msg, keys.Filter):
		filters := []string{"latest", "hot", "new", "top"}
		for i, f := range filters {
			if f == m.filter {
				m.filter = filters[(i+1)%len(filters)]
				break
			}
		}
		m.selected = 0
		m.topics = nil
		m.moreTopicsURL = ""
		return m, m.fetchTopics
	case key.Matches(msg, keys.Refresh):
		m.selected = 0
		m.topics = nil
		m.moreTopicsURL = ""
		return m, m.fetchTopics
	}
	return m, nil
}

func (m Model) updateTopicDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		m.state = topicListView
	case key.Matches(msg, keys.Open):
		if m.topicDetail != nil {
			openInBrowser(fmt.Sprintf("https://linux.do/t/%d", m.topicDetail.ID))
		}
	case key.Matches(msg, keys.Reply):
		m.state = composerView
		m.replyToPost = 0
		m.composer.Reset()
		m.composer.Focus()
		return m, textarea.Blink
	case key.Matches(msg, keys.Like):
		if len(m.posts) > m.currentPostIdx {
			post := m.posts[m.currentPostIdx]
			if m.isLiked(post) {
				return m, m.unlikePost(post.ID)
			}
			return m, m.likePost(post.ID)
		}
	case key.Matches(msg, keys.LoadMore):
		// 加载更多回复
		return m, m.loadMorePosts()
	case key.Matches(msg, keys.Jump):
		// 进入跳转输入模式
		m.state = jumpInputView
		m.jumpInput.Reset()
		m.jumpInput.Focus()
		return m, textarea.Blink
	case key.Matches(msg, keys.Last):
		// 跳转到最后一条
		return m, m.jumpToLast()
	}
	return m, nil
}

func (m Model) updateComposer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = topicDetailView
		m.composer.Reset()
		return m, nil
	case tea.KeyCtrlD:
		content := strings.TrimSpace(m.composer.Value())
		if len(content) > 0 {
			m.state = topicDetailView
			return m, m.createPost(m.topicDetail.ID, content, m.replyToPost)
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		return m, cmd
	}
}

func (m Model) updateJumpInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = topicDetailView
		m.jumpInput.Reset()
		return m, nil
	case tea.KeyEnter:
		input := strings.TrimSpace(m.jumpInput.Value())
		if input != "" {
			floor, err := strconv.Atoi(input)
			if err == nil && floor > 0 {
				m.state = topicDetailView
				m.jumpInput.Reset()
				return m, m.jumpToFloor(floor)
			}
		}
		m.state = topicDetailView
		m.jumpInput.Reset()
		return m, nil
	default:
		var cmd tea.Cmd
		m.jumpInput, cmd = m.jumpInput.Update(msg)
		return m, cmd
	}
}

func (m Model) View() string {
	if !m.ready {
		return "\n  初始化中..."
	}

	switch m.state {
	case topicListView:
		return m.renderTopicList()
	case topicDetailView:
		return m.renderTopicView()
	case composerView:
		return m.renderComposer()
	case jumpInputView:
		return m.renderJumpInput()
	}

	return ""
}

func getFilterEmoji(filter string) string {
	switch filter {
	case "latest":
		return "🆕"
	case "hot":
		return "🔥"
	case "new":
		return "✨"
	case "top":
		return "📊"
	default:
		return "📝"
	}
}

func (m Model) renderTopicList() string {
	var s strings.Builder

	emoji := getFilterEmoji(m.filter)
	title := fmt.Sprintf(" %s Linux.do - %s ", emoji, m.filter)
	s.WriteString(titleStyle.Render(title) + "\n\n")

	if m.err != nil {
		s.WriteString(fmt.Sprintf("❌ 错误: %v\n\n", m.err))
	}

	maxVisible := m.height - 8
	if maxVisible < 10 {
		maxVisible = 10
	}

	start := 0
	end := len(m.topics)
	
	if len(m.topics) > maxVisible {
		halfVisible := maxVisible / 2
		start = m.selected - halfVisible
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(m.topics) {
			end = len(m.topics)
			start = end - maxVisible
			if start < 0 {
				start = 0
			}
		}
	}

	for i := start; i < end && i < len(m.topics); i++ {
		topic := m.topics[i]
		
		titleWidth := 50
		if m.width > 100 {
			titleWidth = m.width - 50
		}
		
		truncatedTitle := truncate(topic.Title, titleWidth)
		paddedTitle := padRight(truncatedTitle, titleWidth)
		
		line := fmt.Sprintf("%3d. %s  ↩️ 回复 %4d  👀 浏览 %6d",
			i+1,
			paddedTitle,
			topic.ReplyCount,
			topic.Views,
		)

		if i == m.selected {
			s.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			s.WriteString(line + "\n")
		}
	}

	s.WriteString("\n")
	
	statusLine := fmt.Sprintf("已加载: %d 条", len(m.topics))
	if m.loading {
		statusLine += " " + loadingStyle.Render("(加载中...)")
	} else if m.moreTopicsURL != "" {
		statusLine += " (按 n 加载更多)"
	} else {
		statusLine += " (已全部加载)"
	}
	s.WriteString(helpStyle.Render(statusLine) + "\n")
	
	helpText := "↑/↓: 移动 | Enter: 打开 | o: 浏览器 | n: 更多 | f: 切换 | g: 刷新 | q: 退出"
	s.WriteString(helpStyle.Render(helpText))

	return s.String()
}

func (m Model) renderTopicView() string {
	if m.topicDetail == nil {
		return "加载中..."
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render(fmt.Sprintf(" 💬 %s ", m.topicDetail.Title)) + "\n\n")
	s.WriteString(m.viewport.View())
	s.WriteString("\n\n")
	
	// 状态行
	currentFloor := 1
	if len(m.posts) > m.currentPostIdx {
		currentFloor = m.posts[m.currentPostIdx].PostNumber
	}
	statusLine := fmt.Sprintf("已加载: %d/%d 楼  当前: %d 楼", len(m.posts), m.topicDetail.PostsCount, currentFloor)
	s.WriteString(helpStyle.Render(statusLine) + "\n")
	
	helpText := "r: 回复 | l: 点赞 | o: 浏览器 | n: 更多 | /: 跳转 | G: 末尾 | ↑/↓: 滚动 | Esc: 返回 | q: 退出"
	s.WriteString(helpStyle.Render(helpText))

	return s.String()
}

func (m Model) renderTopicDetail() string {
	var s strings.Builder

	for i, post := range m.posts {
		header := fmt.Sprintf("👤 @%s  #%d", post.Username, post.PostNumber)
		if i == m.currentPostIdx {
			header = "▶ " + header
		}
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render(header) + "\n\n")
		
		content := htmlToText(post.Cooked)
		s.WriteString(wrapText(content, m.width-8) + "\n")

		if m.isLiked(post) {
			s.WriteString("\n❤️  已点赞\n")
		}

		if i < len(m.posts)-1 {
			s.WriteString("\n" + strings.Repeat("─", min(m.width-4, 100)) + "\n\n")
		}
	}

	return s.String()
}

func (m Model) renderComposer() string {
	var s strings.Builder
	s.WriteString(titleStyle.Render(" ✍️  回复主题 ") + "\n")
	s.WriteString(helpStyle.Render("输入你的回复内容 (支持 Markdown)") + "\n\n")
	s.WriteString(m.composer.View() + "\n\n")
	helpText := "Ctrl+D: 发送 | Esc: 取消"
	s.WriteString(helpStyle.Render(helpText))
	return s.String()
}

func (m Model) renderJumpInput() string {
	var s strings.Builder
	s.WriteString(titleStyle.Render(" 🔍 跳转到指定楼层 ") + "\n\n")
	s.WriteString(fmt.Sprintf("输入楼层号 (1-%d):\n", m.topicDetail.PostsCount))
	s.WriteString(m.jumpInput.View() + "\n\n")
	helpText := "Enter: 跳转 | Esc: 取消"
	s.WriteString(helpStyle.Render(helpText))
	return s.String()
}

func (m Model) isLiked(post client.Post) bool {
	for _, action := range post.ActionsSummary {
		if action.ID == 2 && action.Acted {
			return true
		}
	}
	return false
}

type topicListMsg struct {
	topics  []client.Topic
	users   map[int]string
	err     error
	moreURL string
	append  bool
}

type topicDetailMsg struct {
	detail     *client.TopicDetail
	posts      []client.Post
	allPostIDs []int
	err        error
}

type morePostsMsg struct {
	posts []client.Post
	err   error
}

type jumpToPostMsg struct {
	posts     []client.Post
	targetIdx int
	err       error
}

type postCreatedMsg struct {
	err error
}

func (m Model) fetchTopics() tea.Msg {
	var topics *client.TopicList
	var err error

	switch m.filter {
	case "hot":
		topics, err = m.client.GetHotTopics()
	case "new":
		topics, err = m.client.GetNewTopics()
	case "top":
		topics, err = m.client.GetTopTopics("weekly")
	default:
		topics, err = m.client.GetLatestTopics()
	}

	if err != nil {
		return topicListMsg{err: err, append: false}
	}

	users := make(map[int]string)
	for _, u := range topics.Users {
		users[u.ID] = u.Username
	}

	return topicListMsg{
		topics:  topics.TopicList.Topics,
		users:   users,
		moreURL: topics.TopicList.MoreTopicsURL,
		append:  false,
	}
}

func (m Model) loadMoreTopics() tea.Msg {
	if m.moreTopicsURL == "" {
		return topicListMsg{append: true}
	}

	topics, err := m.client.GetMoreTopics(m.moreTopicsURL)
	if err != nil {
		return topicListMsg{err: err, append: true}
	}

	users := make(map[int]string)
	for _, u := range topics.Users {
		users[u.ID] = u.Username
	}

	return topicListMsg{
		topics:  topics.TopicList.Topics,
		users:   users,
		moreURL: topics.TopicList.MoreTopicsURL,
		append:  true,
	}
}

func (m Model) fetchTopicDetail(topicID int) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.client.GetTopic(topicID)
		if err != nil {
			return topicDetailMsg{err: err}
		}
		return topicDetailMsg{
			detail:     detail,
			posts:      detail.PostStream.Posts,
			allPostIDs: detail.PostStream.Stream,
		}
	}
}

func (m Model) loadMorePosts() tea.Cmd {
	return func() tea.Msg {
		if m.topicDetail == nil || len(m.allPostIDs) == 0 {
			return morePostsMsg{}
		}

		// 获取下一批帖子ID（每次加载20个）
		currentLen := len(m.posts)
		if currentLen >= len(m.allPostIDs) {
			return morePostsMsg{} // 已全部加载
		}

		batchSize := 20
		end := currentLen + batchSize
		if end > len(m.allPostIDs) {
			end = len(m.allPostIDs)
		}

		postIDs := m.allPostIDs[currentLen:end]
		posts, err := m.client.GetPostsByIDs(m.topicDetail.ID, postIDs)
		
		return morePostsMsg{
			posts: posts,
			err:   err,
		}
	}
}

func (m Model) jumpToFloor(floor int) tea.Cmd {
	return func() tea.Msg {
		if m.topicDetail == nil || floor < 1 || floor > m.topicDetail.PostsCount {
			return jumpToPostMsg{err: fmt.Errorf("无效的楼层号")}
		}

		// 检查是否已加载
		for i, post := range m.posts {
			if post.PostNumber == floor {
				return jumpToPostMsg{
					posts:     m.posts,
					targetIdx: i,
				}
			}
		}

		// 未加载，需要获取
		// 找到对应的 postID
		if floor-1 >= len(m.allPostIDs) {
			return jumpToPostMsg{err: fmt.Errorf("楼层号超出范围")}
		}

		// 加载包含目标楼层的上下文（前后各10条）
		start := floor - 11
		if start < 0 {
			start = 0
		}
		end := floor + 10
		if end > len(m.allPostIDs) {
			end = len(m.allPostIDs)
		}

		postIDs := m.allPostIDs[start:end]
		posts, err := m.client.GetPostsByIDs(m.topicDetail.ID, postIDs)
		if err != nil {
			return jumpToPostMsg{err: err}
		}

		// 找到目标索引
		targetIdx := 0
		for i, post := range posts {
			if post.PostNumber == floor {
				targetIdx = i
				break
			}
		}

		return jumpToPostMsg{
			posts:     posts,
			targetIdx: targetIdx,
		}
	}
}

func (m Model) jumpToLast() tea.Cmd {
	return func() tea.Msg {
		if m.topicDetail == nil {
			return jumpToPostMsg{}
		}

		return m.jumpToFloor(m.topicDetail.PostsCount)()
	}
}

func (m Model) createPost(topicID int, content string, replyTo int) tea.Cmd {
	return func() tea.Msg {
		err := m.client.CreatePost(topicID, content, replyTo)
		return postCreatedMsg{err: err}
	}
}

func (m Model) likePost(postID int) tea.Cmd {
	return func() tea.Msg {
		m.client.LikePost(postID)
		return m.fetchTopicDetail(m.topicDetail.ID)()
	}
}

func (m Model) unlikePost(postID int) tea.Cmd {
	return func() tea.Msg {
		m.client.UnlikePost(postID)
		return m.fetchTopicDetail(m.topicDetail.ID)()
	}
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func padRight(s string, width int) string {
	runes := []rune(s)
	currentWidth := runeWidth(runes)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

func runeWidth(runes []rune) int {
	width := 0
	for _, r := range runes {
		if r < 128 {
			width++
		} else {
			width += 2
		}
	}
	return width
}

func wrapText(text string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line + "\n")
			continue
		}

		runes := []rune(line)
		for len(runes) > 0 {
			if len(runes) <= width {
				result.WriteString(string(runes) + "\n")
				break
			}
			result.WriteString(string(runes[:width]) + "\n")
			runes = runes[width:]
		}
	}

	return result.String()
}

func htmlToText(html string) string {
	html = strings.ReplaceAll(html, "</p>", "\n\n")
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")
	html = strings.ReplaceAll(html, "</li>", "\n")
	
	html = strings.ReplaceAll(html, "<pre>", "\n```\n")
	html = strings.ReplaceAll(html, "</pre>", "\n```\n")
	html = strings.ReplaceAll(html, "<code>", "`")
	html = strings.ReplaceAll(html, "</code>", "`")
	
	html = strings.ReplaceAll(html, "<li>", "• ")
	
	html = strings.ReplaceAll(html, "<blockquote>", "\n> ")
	html = strings.ReplaceAll(html, "</blockquote>", "\n")
	
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	
	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
			}
			prevEmpty = true
		} else {
			cleaned = append(cleaned, line)
			prevEmpty = false
		}
	}
	
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
