# Linux.do MCP Server

这是一个将 Linux.do 论坛功能封装为 MCP (Model Context Protocol) Server 的实现，可以让 AI 助手通过标准化的工具接口与 Linux.do 论坛进行交互。

## 功能特性

MCP Server 提供以下工具：

### 1. list_topics - 列出话题
获取论坛话题列表，支持多种过滤方式。

**参数：**
- `filter` (可选): 话题过滤器
  - `latest` - 最新话题（默认）
  - `hot` - 热门话题
  - `new` - 全新话题
  - `top` - 排行榜
- `period` (可选): 当 filter 为 top 时的时间段
  - `daily`, `weekly`（默认）, `monthly`, `quarterly`, `yearly`, `all`

**示例：**
```json
{
  "filter": "hot"
}
```

### 2. get_topic - 获取话题详情
获取指定话题的详细内容和所有帖子。

**参数：**
- `topic_id` (必需): 话题ID

**示例：**
```json
{
  "topic_id": 12345
}
```

### 3. search_posts - 搜索帖子
搜索论坛中的帖子内容。

**参数：**
- `query` (必需): 搜索关键词
- `page` (可选): 页码，默认为 1

**示例：**
```json
{
  "query": "mcp server",
  "page": 1
}
```

### 4. create_post - 创建回帖
在指定话题下创建回帖。

**参数：**
- `topic_id` (必需): 话题ID
- `content` (必需): 回帖内容（支持 Markdown 格式）
- `reply_to_post_number` (可选): 回复的楼层号，0 表示不回复特定楼层

**示例：**
```json
{
  "topic_id": 12345,
  "content": "这是一条测试回复",
  "reply_to_post_number": 0
}
```

### 5. like_post - 点赞帖子
给指定的帖子点赞。

**参数：**
- `post_id` (必需): 帖子ID

**示例：**
```json
{
  "post_id": 67890
}
```

### 6. unlike_post - 取消点赞
取消对指定帖子的点赞。

**参数：**
- `post_id` (必需): 帖子ID

**示例：**
```json
{
  "post_id": 67890
}
```

### 7. get_user_replied_topics - 获取用户回复历史
获取当前用户已回复过的所有话题ID列表。

**参数：** 无

### 8. get_posts - 获取指定帖子
获取话题下的特定帖子内容。

**参数：**
- `topic_id` (必需): 话题ID
- `post_ids` (必需): 帖子ID列表

**示例：**
```json
{
  "topic_id": 12345,
  "post_ids": [1, 2, 3]
}
```

## 安装和构建

### 1. 安装依赖
```bash
cd /path/to/linuxdo
go mod tidy
```

### 2. 构建 MCP Server
```bash
go build -o mcp-server ./cmd/mcp-server
```

## 配置和使用

### 1. 环境变量配置
在运行 MCP Server 之前，需要设置以下环境变量：

```bash
export LINUXDO_USERNAME="your_username"
export LINUXDO_PASSWORD="your_password"
export HTTPS_PROXY="http://127.0.0.1:7890"  # 可选
export NO_PROXY="localhost,127.0.0.1"       # 可选
```

或者在 MCP 客户端配置文件中设置。

说明：MCP Server 会自动读取 `HTTPS_PROXY` / `HTTP_PROXY`（含小写形式），`HTTPS_PROXY` 优先。代理 URL 非法时会直接启动失败，不会降级直连。

### 2. Claude Desktop 配置

编辑 Claude Desktop 的配置文件：

**MacOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

添加以下配置：

```json
{
  "mcpServers": {
    "linuxdo": {
      "command": "/path/to/your/linuxdo/mcp-server",
      "args": [],
      "env": {
        "LINUXDO_USERNAME": "your_username",
        "LINUXDO_PASSWORD": "your_password",
        "HTTPS_PROXY": "http://127.0.0.1:7890"
      }
    }
  }
}
```

### 3. 其他 MCP 客户端配置

对于其他支持 MCP 的客户端（如 Cline、Continue 等），请参考各自的文档添加 MCP Server 配置。

配置格式示例：
```json
{
  "mcpServers": {
    "linuxdo": {
      "command": "/absolute/path/to/mcp-server",
      "env": {
        "LINUXDO_USERNAME": "your_username",
        "LINUXDO_PASSWORD": "your_password",
        "HTTPS_PROXY": "http://127.0.0.1:7890"
      }
    }
  }
}
```

## 使用示例

配置完成后，重启 Claude Desktop，你就可以通过自然语言与 AI 助手交互来使用这些功能：

- "帮我查看 Linux.do 论坛的热门话题"
- "搜索关于 MCP 的帖子"
- "在话题 #12345 下回复：感谢分享！"
- "给帖子 #67890 点赞"
- "查看我回复过的所有话题"

## 直接运行（测试）

你也可以直接运行 MCP Server 进行测试：

```bash
export LINUXDO_USERNAME="your_username"
export LINUXDO_PASSWORD="your_password"
export HTTPS_PROXY="http://127.0.0.1:7890"  # 可选
./mcp-server
```

MCP Server 会通过 stdio 与客户端通信。

## 安全注意事项

1. **不要将包含密码的配置文件提交到版本控制系统**
2. 建议使用环境变量或安全的密钥管理系统存储凭证
3. 定期更新密码并检查账户安全性

## 故障排查

### 登录失败
- 检查用户名和密码是否正确
- 确认环境变量已正确设置
- 查看是否被 Cloudflare 拦截（检查日志输出）

### MCP Server 无法连接
- 确认 MCP Server 程序路径正确
- 检查配置文件格式是否正确
- 查看客户端日志了解详细错误信息

### Cookie 过期
程序会自动处理 Cookie 刷新，如果频繁遇到登录问题，可以删除 `~/.linuxdo_cookies.json` 文件重新登录。

## 技术栈

- Go 1.21+
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP SDK for Go
- [tls-client](https://github.com/bogdanfinn/tls-client) - 绕过 Cloudflare 的 HTTP 客户端
- [bubbletea](https://github.com/charmbracelet/bubbletea) - TUI 框架

## License

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
