# Linux.do Terminal Client

一个用 Go 编写的 Linux.do 论坛终端客户端，支持两种模式：

- **TUI 模式**：全屏图形化终端界面（使用 Bubble Tea 框架）
- **CLI 模式**：命令行摸鱼模式，伪装成普通终端操作

## 功能特性

### TUI 模式
- ✅ 浏览最新/热门/新帖/Top 话题
- ✅ 无限滚动加载更多话题和回复
- ✅ 查看帖子详情和回复
- ✅ 发表回复（支持 Markdown）
- ✅ 点赞/取消点赞
- ✅ 跳转到指定楼层或最后一条回复
- ✅ 在浏览器中打开原帖
- ✅ Cookie 持久化（7天有效期）

### CLI 模式（摸鱼模式）
- ✅ Unix 风格的命令行界面，看起来像在操作服务器
- ✅ 所有 TUI 模式的核心功能
- ✅ 适合在办公环境下使用，不易被发现

## 安装

### 环境要求
- Go 1.21+

### 编译
```bash
go build -o ldo cmd/linuxdo/main.go
```

## 配置

设置环境变量：

```bash
export LINUXDO_USERNAME="your_username"
export LINUXDO_PASSWORD="your_password"
```

或者添加到 `~/.bashrc` / `~/.zshrc`：

```bash
echo 'export LINUXDO_USERNAME="your_username"' >> ~/.bashrc
echo 'export LINUXDO_PASSWORD="your_password"' >> ~/.bashrc
source ~/.bashrc
```

### 系统代理（可选）

客户端会自动读取系统代理环境变量（同时支持大写和小写）：

- `HTTPS_PROXY` / `https_proxy`（访问 `https://` 目标时优先）
- `HTTP_PROXY` / `http_proxy`（`HTTPS_PROXY` 未设置时回退）
- `NO_PROXY` / `no_proxy`（当前仅透传环境变量，不保证与标准库完全一致）

示例（Linux/macOS）：

```bash
export HTTPS_PROXY="http://127.0.0.1:7890"
# 可选：不走代理的域名
export NO_PROXY="localhost,127.0.0.1"
```

示例（Windows PowerShell）：

```powershell
$env:HTTPS_PROXY="http://127.0.0.1:7890"
# 可选：不走代理的域名
$env:NO_PROXY="localhost,127.0.0.1"
```

注意：如果代理 URL 格式非法，程序会在启动时直接报错退出，不会自动降级直连。

## 使用方法

### TUI 模式（默认）

```bash
./ldo
# 或
./ldo --tui
```

#### 快捷键

**话题列表页面：**
- `↑/↓` 或 `k/j` - 上下移动
- `Enter` - 打开选中的话题
- `o` - 在浏览器中打开
- `n` - 加载更多话题
- `f` - 切换过滤器（latest/hot/new/top）
- `g` - 刷新列表
- `q` - 退出

**话题详情页面：**
- `↑/↓` - 滚动查看
- `r` - 回复主题
- `l` - 点赞/取消点赞当前帖子
- `o` - 在浏览器中打开
- `n` - 加载更多回复
- `/` - 跳转到指定楼层
- `G` (Shift+g) - 跳转到最后一条
- `Esc` - 返回话题列表
- `q` - 退出

**回复编辑器：**
- `Ctrl+D` - 发送回复
- `Esc` - 取消

### CLI 模式（摸鱼模式）

```bash
./linuxdo --cli
# 或
export LINUXDO_MODE=cli
./linuxdo
```

#### 可用命令

**导航命令：**
```bash
ls [n]          # 列出话题（可选显示前 n 条）
open <n>        # 打开第 n 个话题
cd <n>          # 切换到话题（同 open）
cd .. / cd      # 返回话题列表
pwd             # 显示当前位置
```

**阅读命令：**
```bash
cat [floor]     # 查看帖子（默认第一楼）
view [floor]    # 查看指定楼层
more            # 加载更多话题/回复
jump <floor>    # 跳转到指定楼层
last            # 跳转到最后一楼
```

**交互命令：**
```bash
reply           # 回复当前话题
like <floor>    # 点赞/取消点赞指定楼层
browser         # 在浏览器中打开
```

**管理命令：**
```bash
filter [name]   # 切换/显示过滤器（latest, hot, new, top）
refresh         # 刷新当前视图
clear           # 清屏
help / ?        # 显示帮助
exit / quit / q # 退出
```

#### 使用示例

```bash
linuxdo> ls 50              # 列出前 50 个话题
linuxdo> open 3             # 打开第 3 个话题
linuxdo> cat 10             # 查看第 10 楼
linuxdo> like 5             # 给第 5 楼点赞
linuxdo> reply              # 回复当前话题
Enter your reply (type 'END' on a new line to finish, 'CANCEL' to cancel):
这是回复内容
支持多行
END
linuxdo> jump 100           # 跳转到第 100 楼
linuxdo> filter hot         # 切换到热门话题
linuxdo> cd ..              # 返回话题列表
linuxdo> exit               # 退出
```

## 技术实现

- **TLS 客户端**：使用 `tls-client` 模拟 Chrome 124 浏览器指纹，绕过 Cloudflare 防护
- **TUI 框架**：使用 `charmbracelet/bubbletea` 构建终端界面
- **Cookie 管理**：自动保存和加载 Cookie，避免重复登录
- **HTML 转文本**：支持代码块、列表、引用等 Markdown 元素
- **中英文混排**：正确计算字符宽度（中文=2，ASCII=1）

## 项目结构

```
.
├── cmd/
│   └── linuxdo/
│       └── main.go           # 主程序入口
├── internal/
│   ├── client/
│   │   └── client.go         # API 客户端和认证
│   ├── ui/
│   │   └── ui.go             # TUI 界面实现
│   └── cli/
│       └── cli.go            # CLI 模式实现
├── archive/                  # 原 Ruby 版本参考
└── README.md
```

## Cookie 存储

Cookie 自动保存在 `~/.linuxdo_cookies.json`，包含：
- 用户名验证
- 7 天有效期
- 自动重新登录

## 认证原理

使用 `bogdanfinn/tls-client` 库模拟 Chrome 124 浏览器特征：
1. 访问首页预热，获取初始 cookies
2. 获取 CSRF token
3. 使用正确的浏览器指纹登录
4. 保持 session cookies 用于后续请求

## 注意事项

1. **账号安全**：不要在公共环境中暴露环境变量
2. **API 频率**：请合理使用，避免频繁请求
3. **摸鱼模式**：CLI 模式仅为伪装界面，请遵守公司规定 😉

## 开发说明

### 依赖管理
```bash
go mod download
go mod tidy
```

### 运行测试
```bash
go test ./...
```

## 参考项目

- [termcourse](https://github.com/merefield/termcourse) - Ruby 版本的 Discourse 终端客户端

- [linuxdo-checkin](https://github.com/doveppp/linuxdo-checkin) -linux.do Daily Check-In. 每日签到，每日打卡

