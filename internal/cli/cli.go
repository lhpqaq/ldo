package cli

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/lhpqaq/ldo/internal/client"
)

type CLI struct {
	client         *client.Client
	currentTopic   *client.TopicDetail
	posts          []client.Post
	allPostIDs     []int
	currentIdx     int
	topics         []client.Topic
	users          map[int]string
	filter         string
	moreURL        string
	reader         *bufio.Reader
	searchResults  []client.SearchResult
	searchQuery    string
	searchPage     int
	isSearchMode   bool
}

func NewCLI(c *client.Client) *CLI {
	return &CLI{
		client: c,
		filter: "latest",
		users:  make(map[int]string),
		reader: bufio.NewReader(os.Stdin),
	}
}

func (c *CLI) Run() {
	fmt.Println("Linux.do Terminal - CLI Mode")
	fmt.Println("Type 'help' for available commands")
	fmt.Println()

	// Load initial topics
	c.loadTopics()

	for {
		fmt.Print("linuxdo> ")
		input, _ := c.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "ls", "list":
			c.cmdList(args)
		case "open":
			c.cmdOpen(args)
		case "cd":
			c.cmdCD(args)
		case "pwd":
			c.cmdPwd()
		case "cat", "view":
			c.cmdView(args)
		case "more":
			c.cmdMore()
		case "reply":
			c.cmdReply()
		case "like":
			c.cmdLike(args)
		case "jump":
			c.cmdJump(args)
		case "last":
			c.cmdLast()
		case "browser":
			c.cmdBrowser()
		case "filter":
			c.cmdFilter(args)
		case "refresh":
			c.cmdRefresh()
		case "search", "find":
			c.cmdSearch(args)
		case "clear":
			fmt.Print("\033[H\033[2J")
		case "bookmarks", "bm":
			c.cmdBookmarks(args)
		case "help", "?":
			c.cmdHelp()
		case "exit", "quit", "q":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			fmt.Println("Type 'help' for available commands")
		}
	}
}

func (c *CLI) loadTopics() {
	var topics *client.TopicList
	var err error

	switch c.filter {
	case "hot":
		topics, err = c.client.GetHotTopics()
	case "new":
		topics, err = c.client.GetNewTopics()
	case "top":
		topics, err = c.client.GetTopTopics("weekly")
	default:
		topics, err = c.client.GetLatestTopics()
	}

	if err != nil {
		fmt.Printf("Error loading topics: %v\n", err)
		return
	}

	c.topics = topics.TopicList.Topics
	c.moreURL = topics.TopicList.MoreTopicsURL
	for _, u := range topics.Users {
		c.users[u.ID] = u.Username
	}
}

func (c *CLI) cmdList(args []string) {
	// 如果在搜索模式，显示搜索结果
	if c.isSearchMode {
		if len(c.searchResults) == 0 {
			fmt.Println("No search results")
			return
		}

		fmt.Printf("\nSearch Results for '%s' (page %d):\n", c.searchQuery, c.searchPage)
		fmt.Println(strings.Repeat("-", 80))

		for i, result := range c.searchResults {
			blurb := result.Blurb
			if len(blurb) > 70 {
				blurb = blurb[:67] + "..."
			}

			fmt.Printf("%3d. @%-15s [Topic #%d, Floor #%d] ❤️ %d\n",
				i+1, result.Username, result.TopicID, result.PostNumber, result.LikeCount)
			fmt.Printf("     %s\n", blurb)
			fmt.Println()
		}

		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("Page %d | Total: %d results\n", c.searchPage, len(c.searchResults))
		return
	}

	// 原来的话题列表显示
	if len(c.topics) == 0 {
		fmt.Println("No topics loaded")
		return
	}

	limit := 20
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			limit = n
		}
	}

	fmt.Printf("Topics (%s):\n", c.filter)
	fmt.Println(strings.Repeat("-", 80))

	for i, topic := range c.topics {
		if i >= limit {
			break
		}
		title := topic.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Printf("%3d. %-50s  Replies: %4d  Views: %6d\n",
			i+1, title, topic.ReplyCount, topic.Views)
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d topics loaded", len(c.topics))
	if c.moreURL != "" {
		fmt.Printf(" (use 'more' to load more)")
	}
	fmt.Println()
}

func (c *CLI) cmdOpen(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: open <number>")
		return
	}

	idx, err := strconv.Atoi(args[0])
	if err != nil || idx < 1 {
		fmt.Printf("Invalid number: %s\n", args[0])
		return
	}

	var topicID int

	// 在搜索模式下，从搜索结果中获取 topicID
	if c.isSearchMode {
		if idx > len(c.searchResults) {
			fmt.Printf("Invalid search result number: %d\n", idx)
			return
		}
		topicID = c.searchResults[idx-1].TopicID
	} else {
		// 正常模式，从话题列表获取
		if idx > len(c.topics) {
			fmt.Printf("Invalid topic number: %d\n", idx)
			return
		}
		topicID = c.topics[idx-1].ID
	}

	detail, err := c.client.GetTopic(topicID)
	if err != nil {
		fmt.Printf("Error loading topic: %v\n", err)
		return
	}

	c.currentTopic = detail
	c.posts = detail.PostStream.Posts
	c.allPostIDs = detail.PostStream.Stream
	c.currentIdx = 0

	fmt.Printf("Opened: %s\n", detail.Title)
	fmt.Printf("Total posts: %d\n", detail.PostsCount)
}

func (c *CLI) cmdCD(args []string) {
	if len(args) == 0 {
		c.currentTopic = nil
		c.posts = nil
		c.allPostIDs = nil
		c.isSearchMode = false
		c.searchResults = nil
		fmt.Println("Back to topic list")
		return
	}

	if args[0] == ".." {
		c.currentTopic = nil
		c.posts = nil
		c.allPostIDs = nil
		c.isSearchMode = false
		c.searchResults = nil
		fmt.Println("Back to topic list")
		return
	}

	c.cmdOpen(args)
}

func (c *CLI) cmdPwd() {
	if c.currentTopic == nil {
		fmt.Println("/topics")
	} else {
		fmt.Printf("/topics/%d - %s\n", c.currentTopic.ID, c.currentTopic.Title)
	}
}

func (c *CLI) cmdView(args []string) {
	if c.currentTopic == nil {
		fmt.Println("No topic opened. Use 'open <number>' first")
		return
	}

	if len(args) == 0 {
		// View first post
		if len(c.posts) > 0 {
			c.displayPost(c.posts[0])
		}
		return
	}

	floor, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("Invalid floor number: %s\n", args[0])
		return
	}

	// Check if already loaded
	for _, post := range c.posts {
		if post.PostNumber == floor {
			c.displayPost(post)
			return
		}
	}

	// Need to load
	if floor < 1 || floor > c.currentTopic.PostsCount {
		fmt.Printf("Invalid floor number: %d\n", floor)
		return
	}

	start := floor - 1
	if start < 0 {
		start = 0
	}
	end := start + 1
	if end > len(c.allPostIDs) {
		end = len(c.allPostIDs)
	}

	postIDs := c.allPostIDs[start:end]
	posts, err := c.client.GetPostsByIDs(c.currentTopic.ID, postIDs)
	if err != nil {
		fmt.Printf("Error loading post: %v\n", err)
		return
	}

	if len(posts) > 0 {
		c.displayPost(posts[0])
	}
}

func (c *CLI) displayPost(post client.Post) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Floor #%d | Author: @%s | Time: %s\n", post.PostNumber, post.Username, post.CreatedAt)
	fmt.Println(strings.Repeat("-", 80))

	content := htmlToText(post.Cooked)
	fmt.Println(content)

	fmt.Println(strings.Repeat("=", 80))

	// Check if liked
	for _, action := range post.ActionsSummary {
		if action.ID == 2 && action.Acted {
			fmt.Println("Status: Liked")
		}
	}
}

func (c *CLI) cmdMore() {
	if c.currentTopic == nil {
		// 在搜索模式下，加载下一页搜索结果
		if c.isSearchMode {
			c.searchPage++
			c.performSearch(c.searchQuery, c.searchPage)
			return
		}

		// Load more topics
		if c.moreURL == "" {
			fmt.Println("No more topics to load")
			return
		}

		topics, err := c.client.GetMoreTopics(c.moreURL)
		if err != nil {
			fmt.Printf("Error loading more topics: %v\n", err)
			return
		}

		c.topics = append(c.topics, topics.TopicList.Topics...)
		c.moreURL = topics.TopicList.MoreTopicsURL
		for _, u := range topics.Users {
			c.users[u.ID] = u.Username
		}

		fmt.Printf("Loaded %d more topics. Total: %d\n", len(topics.TopicList.Topics), len(c.topics))
	} else {
		// Load more posts
		currentLen := len(c.posts)
		if currentLen >= len(c.allPostIDs) {
			fmt.Println("All posts loaded")
			return
		}

		batchSize := 20
		end := currentLen + batchSize
		if end > len(c.allPostIDs) {
			end = len(c.allPostIDs)
		}

		postIDs := c.allPostIDs[currentLen:end]
		posts, err := c.client.GetPostsByIDs(c.currentTopic.ID, postIDs)
		if err != nil {
			fmt.Printf("Error loading posts: %v\n", err)
			return
		}

		c.posts = append(c.posts, posts...)
		fmt.Printf("Loaded %d more posts. Total: %d/%d\n", len(posts), len(c.posts), c.currentTopic.PostsCount)
	}
}

func (c *CLI) cmdReply() {
	if c.currentTopic == nil {
		fmt.Println("No topic opened")
		return
	}

	fmt.Println("Enter your reply (type 'END' on a new line to finish, 'CANCEL' to cancel):")
	var lines []string
	for {
		line, _ := c.reader.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line == "END" {
			break
		}
		if line == "CANCEL" {
			fmt.Println("Reply cancelled")
			return
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	content = strings.TrimSpace(content)

	if content == "" {
		fmt.Println("Empty reply, cancelled")
		return
	}

	err := c.client.CreatePost(c.currentTopic.ID, content, 0)
	if err != nil {
		fmt.Printf("Error posting reply: %v\n", err)
		return
	}

	fmt.Println("Reply posted successfully!")
}

func (c *CLI) cmdLike(args []string) {
	if c.currentTopic == nil {
		fmt.Println("No topic opened")
		return
	}

	if len(args) == 0 {
		fmt.Println("Usage: like <floor_number>")
		return
	}

	floor, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("Invalid floor number: %s\n", args[0])
		return
	}

	// Find the post
	var targetPost *client.Post
	for _, post := range c.posts {
		if post.PostNumber == floor {
			targetPost = &post
			break
		}
	}

	if targetPost == nil {
		fmt.Printf("Floor %d not loaded yet\n", floor)
		return
	}

	// Check if already liked
	isLiked := false
	for _, action := range targetPost.ActionsSummary {
		if action.ID == 2 && action.Acted {
			isLiked = true
			break
		}
	}

	if isLiked {
		err = c.client.UnlikePost(targetPost.ID)
		if err != nil {
			fmt.Printf("Error unliking post: %v\n", err)
			return
		}
		fmt.Println("Post unliked")
	} else {
		err = c.client.LikePost(targetPost.ID)
		if err != nil {
			fmt.Printf("Error liking post: %v\n", err)
			return
		}
		fmt.Println("Post liked!")
	}
}

func (c *CLI) cmdJump(args []string) {
	if c.currentTopic == nil {
		fmt.Println("No topic opened")
		return
	}

	if len(args) == 0 {
		fmt.Println("Usage: jump <floor_number>")
		return
	}

	c.cmdView(args)
}

func (c *CLI) cmdLast() {
	if c.currentTopic == nil {
		fmt.Println("No topic opened")
		return
	}

	floor := c.currentTopic.PostsCount
	c.cmdView([]string{strconv.Itoa(floor)})
}

func (c *CLI) cmdBrowser() {
	if c.currentTopic == nil {
		fmt.Println("No topic opened")
		return
	}

	url := fmt.Sprintf("https://linux.do/t/%d", c.currentTopic.ID)
	fmt.Printf("Opening in browser: %s\n", url)

	// Try to open in browser (simple approach)
	fmt.Println("(Browser opening not implemented in CLI mode)")
}

func (c *CLI) cmdFilter(args []string) {
	if len(args) == 0 {
		fmt.Printf("Current filter: %s\n", c.filter)
		fmt.Println("Available filters: latest, hot, new, top")
		return
	}

	filter := args[0]
	validFilters := []string{"latest", "hot", "new", "top"}
	valid := false
	for _, f := range validFilters {
		if f == filter {
			valid = true
			break
		}
	}

	if !valid {
		fmt.Printf("Invalid filter: %s\n", filter)
		fmt.Println("Available filters: latest, hot, new, top")
		return
	}

	c.filter = filter
	c.topics = nil
	c.moreURL = ""
	c.loadTopics()
	fmt.Printf("Switched to %s filter\n", c.filter)
}

func (c *CLI) cmdRefresh() {
	if c.currentTopic != nil {
		// Refresh current topic
		detail, err := c.client.GetTopic(c.currentTopic.ID)
		if err != nil {
			fmt.Printf("Error refreshing topic: %v\n", err)
			return
		}
		c.currentTopic = detail
		c.posts = detail.PostStream.Posts
		c.allPostIDs = detail.PostStream.Stream
		fmt.Println("Topic refreshed")
	} else if c.isSearchMode {
		// Refresh search results
		c.performSearch(c.searchQuery, c.searchPage)
	} else {
		// Refresh topic list
		c.topics = nil
		c.moreURL = ""
		c.loadTopics()
		fmt.Println("Topic list refreshed")
	}
}

func (c *CLI) cmdSearch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: search <query>")
		fmt.Println("Example: search mcp")
		return
	}

	query := strings.Join(args, " ")
	c.searchQuery = query
	c.searchPage = 1
	c.isSearchMode = true
	c.performSearch(query, 1)
}

func (c *CLI) performSearch(query string, page int) {
	fmt.Printf("Searching for '%s' (page %d)...\n", query, page)

	results, err := c.client.Search(query, page)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	c.searchResults = results.Posts
	c.searchPage = page

	if len(results.Posts) == 0 {
		fmt.Println("No results found")
		return
	}

	fmt.Printf("\nSearch Results (%d results):\n", len(results.Posts))
	fmt.Println(strings.Repeat("-", 80))

	for i, result := range results.Posts {
		blurb := result.Blurb
		if len(blurb) > 70 {
			blurb = blurb[:67] + "..."
		}

		fmt.Printf("%3d. @%-15s [Topic #%d, Floor #%d] ❤️ %d\n",
			i+1, result.Username, result.TopicID, result.PostNumber, result.LikeCount)
		fmt.Printf("     %s\n", blurb)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Page %d | Use 'open <n>' to view topic | 'more' for next page | 'cd ..' to exit search\n", page)
}

func (c *CLI) cmdHelp() {
	help := `
Available Commands:

Navigation:
  ls [n]          - List topics (optionally show first n topics)
  open <n>        - Open topic by number
  cd <n>          - Change to topic (same as open)
  cd .. / cd      - Go back to topic list
  pwd             - Show current location

Search:
  search <query>  - Search posts by keyword
  find <query>    - Alias for search
  more            - Load next page of search results

Reading:
  cat [floor]     - View post (default: first post)
  view [floor]    - View post by floor number
  more            - Load more topics/posts
  jump <floor>    - Jump to specific floor
  last            - Jump to last post

Interaction:
  reply           - Reply to current topic
  like <floor>    - Like/unlike a post
  browser         - Open current topic in browser

Management:
  filter [name]   - Change/show filter (latest, hot, new, top)
  refresh         - Refresh current view
  clear           - Clear screen

Bookmarks:
  bookmarks / bm  - List all bookmarks
  bm export [fmt] - Export bookmarks (txt, html, md) to ~/下載/
  bm clear        - Delete all bookmarks (with confirmation)

  help / ?        - Show this help
  exit / quit / q - Exit

Examples:
  search mcp      - Search for posts containing "mcp"
  ls 50           - List first 50 topics
  open 3          - Open topic #3
  cat 10          - View floor #10
  like 5          - Like floor #5
  filter hot      - Switch to hot topics
`
	fmt.Println(help)
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

// cmdBookmarks 書籤管理命令
func (c *CLI) cmdBookmarks(args []string) {
	if len(args) == 0 {
		c.listBookmarks()
		return
	}

	switch args[0] {
	case "export":
		format := "md"
		if len(args) > 1 {
			format = strings.ToLower(args[1])
		}
		c.exportBookmarks(format)
	case "clear":
		c.clearBookmarks()
	default:
		fmt.Printf("Unknown bookmarks subcommand: %s\n", args[0])
		fmt.Println("Usage: bookmarks [export <txt|html|md> | clear]")
	}
}

func (c *CLI) listBookmarks() {
	fmt.Println("Fetching bookmarks...")
	bookmarks, err := c.client.GetAllBookmarks()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks found.")
		return
	}

	fmt.Printf("\nFound %d bookmarks:\n\n", len(bookmarks))
	for i, bm := range bookmarks {
		title := bm.Title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		fmt.Printf("%3d. %s\n", i+1, title)
		fmt.Printf("     %s\n", bm.CreatedAt[:10])
	}
	fmt.Println()
}

func (c *CLI) exportBookmarks(format string) {
	if format != "txt" && format != "html" && format != "md" {
		fmt.Printf("Invalid format: %s\n", format)
		fmt.Println("Supported formats: txt, html, md")
		return
	}

	fmt.Println("Fetching bookmarks...")
	bookmarks, err := c.client.GetAllBookmarks()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks to export.")
		return
	}

	outputDir := "/home/joe/下載"
	filename := fmt.Sprintf("bookmarks.%s", format)
	filepath := fmt.Sprintf("%s/%s", outputDir, filename)

	var content string
	switch format {
	case "txt":
		content = c.formatBookmarksTXT(bookmarks)
	case "html":
		content = c.formatBookmarksHTML(bookmarks)
	case "md":
		content = c.formatBookmarksMD(bookmarks)
	}

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("Exported %d bookmarks to: %s\n", len(bookmarks), filepath)
}

func (c *CLI) formatBookmarksTXT(bookmarks []client.Bookmark) string {
	var sb strings.Builder
	sb.WriteString("Linux.do Bookmarks\n")
	sb.WriteString("==================\n\n")

	for i, bm := range bookmarks {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, bm.Title))
		sb.WriteString(fmt.Sprintf("   URL: https://linux.do%s\n", bm.BookmarkableURL))
		sb.WriteString(fmt.Sprintf("   Date: %s\n", bm.CreatedAt[:10]))
		if bm.Excerpt != "" {
			excerpt := bm.Excerpt
			if len(excerpt) > 200 {
				excerpt = excerpt[:197] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", excerpt))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (c *CLI) formatBookmarksHTML(bookmarks []client.Bookmark) string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"zh-TW\">\n<head>\n")
	sb.WriteString("  <meta charset=\"UTF-8\">\n")
	sb.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString("  <title>Linux.do Bookmarks</title>\n")
	sb.WriteString("  <style>\n")
	sb.WriteString("    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }\n")
	sb.WriteString("    h1 { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }\n")
	sb.WriteString("    .bookmark { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 8px; }\n")
	sb.WriteString("    .bookmark:hover { box-shadow: 0 2px 8px rgba(0,0,0,0.1); }\n")
	sb.WriteString("    .title { font-size: 1.1em; font-weight: bold; margin-bottom: 8px; }\n")
	sb.WriteString("    .title a { color: #007bff; text-decoration: none; }\n")
	sb.WriteString("    .title a:hover { text-decoration: underline; }\n")
	sb.WriteString("    .date { color: #666; font-size: 0.9em; }\n")
	sb.WriteString("    .excerpt { color: #444; margin-top: 10px; line-height: 1.5; }\n")
	sb.WriteString("  </style>\n</head>\n<body>\n")
	sb.WriteString(fmt.Sprintf("  <h1>Linux.do Bookmarks (%d)</h1>\n", len(bookmarks)))

	for _, bm := range bookmarks {
		url := fmt.Sprintf("https://linux.do%s", bm.BookmarkableURL)
		sb.WriteString("  <div class=\"bookmark\">\n")
		sb.WriteString(fmt.Sprintf("    <div class=\"title\"><a href=\"%s\" target=\"_blank\">%s</a></div>\n", url, bm.Title))
		sb.WriteString(fmt.Sprintf("    <div class=\"date\">%s</div>\n", bm.CreatedAt[:10]))
		if bm.Excerpt != "" {
			sb.WriteString(fmt.Sprintf("    <div class=\"excerpt\">%s</div>\n", bm.Excerpt))
		}
		sb.WriteString("  </div>\n")
	}

	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

func (c *CLI) formatBookmarksMD(bookmarks []client.Bookmark) string {
	var sb strings.Builder
	sb.WriteString("# Linux.do Bookmarks\n\n")
	sb.WriteString(fmt.Sprintf("Total: %d bookmarks\n\n", len(bookmarks)))
	sb.WriteString("---\n\n")

	for i, bm := range bookmarks {
		url := fmt.Sprintf("https://linux.do%s", bm.BookmarkableURL)
		sb.WriteString(fmt.Sprintf("## %d. [%s](%s)\n\n", i+1, bm.Title, url))
		sb.WriteString(fmt.Sprintf("**Date**: %s\n\n", bm.CreatedAt[:10]))
		if bm.Excerpt != "" {
			sb.WriteString(fmt.Sprintf("> %s\n\n", bm.Excerpt))
		}
		sb.WriteString("---\n\n")
	}

	return sb.String()
}

func (c *CLI) clearBookmarks() {
	fmt.Print("Are you sure you want to delete ALL bookmarks? (yes/no): ")
	input, _ := c.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "yes" && input != "y" {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println("Fetching bookmarks...")
	bookmarks, err := c.client.GetAllBookmarks()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks to delete.")
		return
	}

	fmt.Printf("Deleting %d bookmarks...\n", len(bookmarks))
	deleted := 0
	for i, bm := range bookmarks {
		if err := c.client.DeleteBookmark(bm.ID); err != nil {
			fmt.Printf("Failed to delete bookmark %d: %v\n", bm.ID, err)
			continue
		}
		deleted++
		fmt.Printf("\rDeleted %d/%d", i+1, len(bookmarks))
	}
	fmt.Printf("\nSuccessfully deleted %d bookmarks.\n", deleted)
}
