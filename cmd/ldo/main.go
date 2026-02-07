package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lhpqaq/ldo/internal/cli"
	"github.com/lhpqaq/ldo/internal/client"
	"github.com/lhpqaq/ldo/internal/ui"
)

func main() {
	username := os.Getenv("LINUXDO_USERNAME")
	password := os.Getenv("LINUXDO_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("请设置 LINUXDO_USERNAME 和 LINUXDO_PASSWORD 环境变量")
	}

	// Determine mode: CLI or TUI
	mode := os.Getenv("LINUXDO_MODE")
	if len(os.Args) > 1 {
		if os.Args[1] == "--cli" || os.Args[1] == "-c" {
			mode = "cli"
		} else if os.Args[1] == "--tui" || os.Args[1] == "-t" {
			mode = "tui"
		}
	}

	fmt.Println("正在连接 Linux.do 论坛...")

	c, err := client.NewClient("https://linux.do", username, password)
	if err != nil {
		log.Fatalf("客户端初始化失败: %v", err)
	}

	fmt.Printf("✅ 登录成功! 用户: %s\n", c.GetUsername())

	if mode == "cli" {
		fmt.Println("启动 CLI 摸鱼模式...")
		cliMode := cli.NewCLI(c)
		cliMode.Run()
	} else {
		fmt.Println("启动 TUI 终端界面...")
		p := tea.NewProgram(
			ui.NewModel(c),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
	}
}
