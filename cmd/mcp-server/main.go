package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/lhpqaq/ldo/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type LinuxDoServer struct {
	client *client.Client
	initErr error
}

func main() {
	username := os.Getenv("LINUXDO_USERNAME")
	password := os.Getenv("LINUXDO_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("请设置 LINUXDO_USERNAME 和 LINUXDO_PASSWORD 环境变量")
	}

	// 记录到 stderr，这样 Claude Desktop 可以看到日志
	fmt.Fprintf(os.Stderr, "正在初始化 Linux.do 客户端...\n")
	fmt.Fprintf(os.Stderr, "用户名: %s\n", username)

	// 初始化客户端（会尝试使用已保存的登录状态）
	c, err := client.NewClient("https://linux.do", username, password)

	ldoServer := &LinuxDoServer{
		client: c,
		initErr: err,
	}

	if err != nil {
		// 登录失败也不退出，而是记录错误，后续调用时返回错误信息
		fmt.Fprintf(os.Stderr, "⚠️ 客户端初始化失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "MCP Server 将继续运行，但工具调用会失败\n")
	} else {
		fmt.Fprintf(os.Stderr, "✅ 客户端初始化成功！用户: %s\n", c.GetUsername())
	}

	// 创建 MCP server
	mcpServer := server.NewMCPServer(
		"linuxdo-forum",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 注册工具
	ldoServer.registerTools(mcpServer)

	fmt.Fprintf(os.Stderr, "MCP Server 已启动，等待连接...\n")

	// 启动 stdio server
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatal(err)
	}
}

func (s *LinuxDoServer) registerTools(mcpServer *server.MCPServer) {
	// 1. 列出话题
	mcpServer.AddTool(mcp.Tool{
		Name:        "list_topics",
		Description: "获取Linux.do论坛话题列表，支持多种过滤方式（latest/hot/new/top）",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "话题过滤器：latest(最新)、hot(热门)、new(全新)、top(排行)",
				},
				"period": map[string]interface{}{
					"type":        "string",
					"description": "时间段，仅filter为top时有效：daily、weekly、monthly、quarterly、yearly、all",
				},
			},
		},
	}, s.handleListTopics)

	// 2. 获取话题详情
	mcpServer.AddTool(mcp.Tool{
		Name:        "get_topic",
		Description: "获取指定话题的详细内容和所有帖子",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"topic_id": map[string]interface{}{
					"type":        "number",
					"description": "话题ID",
				},
			},
			Required: []string{"topic_id"},
		},
	}, s.handleGetTopic)

	// 3. 搜索帖子
	mcpServer.AddTool(mcp.Tool{
		Name:        "search_posts",
		Description: "搜索论坛帖子内容",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词",
				},
				"page": map[string]interface{}{
					"type":        "number",
					"description": "页码，默认为1",
				},
			},
			Required: []string{"query"},
		},
	}, s.handleSearch)

	// 4. 创建回帖
	mcpServer.AddTool(mcp.Tool{
		Name:        "create_post",
		Description: "在指定话题下创建回帖，支持Markdown格式",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"topic_id": map[string]interface{}{
					"type":        "number",
					"description": "话题ID",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "回帖内容（支持Markdown）",
				},
				"reply_to_post_number": map[string]interface{}{
					"type":        "number",
					"description": "回复的楼层号，0表示不回复特定楼层",
				},
			},
			Required: []string{"topic_id", "content"},
		},
	}, s.handleCreatePost)

	// 5. 点赞帖子
	mcpServer.AddTool(mcp.Tool{
		Name:        "like_post",
		Description: "给指定的帖子点赞",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"post_id": map[string]interface{}{
					"type":        "number",
					"description": "帖子ID",
				},
			},
			Required: []string{"post_id"},
		},
	}, s.handleLikePost)

	// 6. 取消点赞
	mcpServer.AddTool(mcp.Tool{
		Name:        "unlike_post",
		Description: "取消对指定帖子的点赞",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"post_id": map[string]interface{}{
					"type":        "number",
					"description": "帖子ID",
				},
			},
			Required: []string{"post_id"},
		},
	}, s.handleUnlikePost)

	// 7. 获取用户已回复的话题
	mcpServer.AddTool(mcp.Tool{
		Name:        "get_user_replied_topics",
		Description: "获取当前用户已回复过的所有话题ID列表",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleGetUserRepliedTopics)

	// 8. 获取帖子内容
	mcpServer.AddTool(mcp.Tool{
		Name:        "get_posts",
		Description: "获取指定话题下的特定帖子内容（批量获取）",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"topic_id": map[string]interface{}{
					"type":        "number",
					"description": "话题ID",
				},
				"post_ids": map[string]interface{}{
					"type":        "array",
					"description": "帖子ID列表",
					"items": map[string]interface{}{
						"type": "number",
					},
				},
			},
			Required: []string{"topic_id", "post_ids"},
		},
	}, s.handleGetPosts)
}

// 检查客户端是否可用
func (s *LinuxDoServer) checkClient() error {
	if s.initErr != nil {
		return fmt.Errorf("客户端初始化失败: %w", s.initErr)
	}
	if s.client == nil {
		return fmt.Errorf("客户端未初始化")
	}
	return nil
}

// Handler implementations
func (s *LinuxDoServer) handleListTopics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		Filter string `json:"filter"`
		Period string `json:"period"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	if params.Filter == "" {
		params.Filter = "latest"
	}
	if params.Period == "" {
		params.Period = "weekly"
	}

	var topics *client.TopicList
	var err error

	switch params.Filter {
	case "hot":
		topics, err = s.client.GetHotTopics()
	case "new":
		topics, err = s.client.GetNewTopics()
	case "top":
		topics, err = s.client.GetTopTopics(params.Period)
	default:
		topics, err = s.client.GetLatestTopics()
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取话题列表失败: %v", err)), nil
	}

	result, _ := json.MarshalIndent(topics, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

func (s *LinuxDoServer) handleGetTopic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		TopicID float64 `json:"topic_id"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	topic, err := s.client.GetTopic(int(params.TopicID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取话题失败: %v", err)), nil
	}

	result, _ := json.MarshalIndent(topic, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

func (s *LinuxDoServer) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		Query string  `json:"query"`
		Page  float64 `json:"page"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	page := int(params.Page)
	if page == 0 {
		page = 1
	}

	searchResult, err := s.client.Search(params.Query, page)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("搜索失败: %v", err)), nil
	}

	result, _ := json.MarshalIndent(searchResult, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

func (s *LinuxDoServer) handleCreatePost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		TopicID           float64 `json:"topic_id"`
		Content           string  `json:"content"`
		ReplyToPostNumber float64 `json:"reply_to_post_number"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	err := s.client.CreatePost(int(params.TopicID), params.Content, int(params.ReplyToPostNumber))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("发帖失败: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("✅ 成功在话题 #%d 发布回帖", int(params.TopicID))), nil
}

func (s *LinuxDoServer) handleLikePost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		PostID float64 `json:"post_id"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	err := s.client.LikePost(int(params.PostID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("点赞失败: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("✅ 成功点赞帖子 #%d", int(params.PostID))), nil
}

func (s *LinuxDoServer) handleUnlikePost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		PostID float64 `json:"post_id"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	err := s.client.UnlikePost(int(params.PostID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("取消点赞失败: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("✅ 成功取消点赞帖子 #%d", int(params.PostID))), nil
}

func (s *LinuxDoServer) handleGetUserRepliedTopics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	topics, err := s.client.GetUserRepliedTopics()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取用户回复历史失败: %v", err)), nil
	}

	topicIDs := make([]int, 0, len(topics))
	for id := range topics {
		topicIDs = append(topicIDs, id)
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"replied_topic_ids": topicIDs,
		"total_count":       len(topicIDs),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *LinuxDoServer) handleGetPosts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.checkClient(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var params struct {
		TopicID float64   `json:"topic_id"`
		PostIDs []float64 `json:"post_ids"`
	}

	argsBytes, _ := json.Marshal(request.Params.Arguments)
	if err := json.Unmarshal(argsBytes, &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("参数解析失败: %v", err)), nil
	}

	// Convert float64 to int
	postIDs := make([]int, len(params.PostIDs))
	for i, id := range params.PostIDs {
		postIDs[i] = int(id)
	}

	posts, err := s.client.GetPostsByIDs(int(params.TopicID), postIDs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("获取帖子失败: %v", err)), nil
	}

	result, _ := json.MarshalIndent(posts, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}
