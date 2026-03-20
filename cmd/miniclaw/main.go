package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"miniclaw-go/internal/app"
)

func main() {
	var (
		configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
		sessionID  = flag.String("session", "demo", "会话 ID")
		once       = flag.String("once", "", "单次执行输入后退出")
	)
	flag.Parse()

	application, err := app.New(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动失败：%v\n", err)
		os.Exit(1)
	}
	defer application.Close()

	for _, warning := range application.StartupWarnings() {
		fmt.Printf("[警告] %s\n", warning)
	}

	if strings.TrimSpace(*once) != "" {
		runOnce(application, *sessionID, *once)
		return
	}

	runREPL(application, *sessionID)
}

func runREPL(application *app.App, sessionID string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("MiniClaw-Go 交互模式（session=%s）\n", sessionID)
	fmt.Println("输入消息，或使用 /tools、/skills、/memory、/history、/exit")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if handled := handleSlashCommand(application, sessionID, input); handled {
			if input == "/exit" {
				return
			}
			continue
		}
		runOnce(application, sessionID, input)
	}
}

func handleSlashCommand(application *app.App, sessionID, input string) bool {
	switch input {
	case "/tools":
		for _, tool := range application.ListTools() {
			fmt.Printf("- %s [%s]: %s\n", tool.Name, tool.Source, tool.Description)
		}
		return true
	case "/skills":
		skills, err := application.ListSkills()
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			return true
		}
		for _, skill := range skills {
			fmt.Printf("- %s (%s): %s\n", skill.Name, skill.Path, skill.UseWhen)
		}
		return true
	case "/memory":
		files, err := application.ListMemoryFiles()
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			return true
		}
		if len(files) == 0 {
			fmt.Println("（当前没有记忆文件）")
			return true
		}
		for _, file := range files {
			fmt.Printf("- %s\n", file)
		}
		return true
	case "/history":
		stats, err := application.SessionStats(sessionID)
		if err != nil {
			fmt.Printf("错误：%v\n", err)
			return true
		}
		fmt.Printf("session=%s history=%d summary=%t last=%s\n", stats.SessionID, stats.HistoryCount, stats.SummaryExists, stats.LastActivityAt)
		return true
	case "/exit":
		return true
	default:
		return false
	}
}

func runOnce(application *app.App, sessionID, input string) {
	result, err := application.Run(context.Background(), sessionID, input)
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		return
	}

	fmt.Printf("\n%s\n", strings.TrimSpace(result.Output))
	if len(result.UsedTools) > 0 {
		fmt.Printf("工具：%s\n", strings.Join(result.UsedTools, ", "))
	} else {
		fmt.Println("工具：（无）")
	}
	if len(result.UsedSkills) > 0 {
		fmt.Printf("技能：%s\n", strings.Join(result.UsedSkills, ", "))
	} else {
		fmt.Println("技能：（无）")
	}
	fmt.Printf("追踪：%s\n\n", result.TracePath)
}
