package agent

import (
	"fmt"
	"strings"

	"miniclaw-go/internal/core"
)

type PromptSection struct {
	Title   string
	Content string
}

type PromptInput struct {
	CoreDocs       []PromptSection
	MemoryContext  string
	SessionSummary string
	RecentHistory  []core.Message
	Tools          []core.ToolDescriptor
	Skills         []core.SkillDescriptor
}

func BuildSystemPrompt(input PromptInput) string {
	var b strings.Builder
	b.WriteString("你是 MiniClaw-Go，一个透明、可审计的本地 CLI Agent。\n")
	b.WriteString("请以 ReAct 风格循环工作：要么直接回答，要么一次只调用一个工具。\n")
	b.WriteString("如果某个 skill 与任务相关，在使用其他工具前先用 read_file 阅读它的 SKILL.md。\n")
	b.WriteString("回答要简洁、务实，并明确说明结果。\n")
	b.WriteString("绝不要声称完成了你实际上没有完成的事情。\n")
	b.WriteString("当用户要求你记住某件事时，请在记忆真正写入后再确认。\n")

	for _, section := range input.CoreDocs {
		writeSection(&b, section.Title, section.Content)
	}

	writeSection(&b, "记忆上下文", input.MemoryContext)
	writeSection(&b, "会话摘要", input.SessionSummary)

	b.WriteString("\n## 工具列表\n")
	if len(input.Tools) == 0 {
		b.WriteString("- 当前没有可用工具。\n")
	} else {
		for _, tool := range input.Tools {
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", tool.Name, tool.Source, tool.Description))
		}
	}

	b.WriteString("\n## 技能列表\n")
	if len(input.Skills) == 0 {
		b.WriteString("- 当前没有可用技能。\n")
	} else {
		for _, skill := range input.Skills {
			b.WriteString(fmt.Sprintf("- %s [%s]: %s\n", skill.Name, skill.Path, skill.UseWhen))
		}
	}

	return strings.TrimSpace(b.String())
}

// EstimateTokens 使用简单近似估算 token 数：约 4 个字符视为 1 个 token。
func EstimateTokens(content string) int {
	return len(content) / 4
}

func EstimateHistoryTokens(history []core.Message) int {
	total := 0
	for _, msg := range history {
		total += EstimateTokens(msg.Content)
		for _, call := range msg.ToolCalls {
			total += EstimateTokens(call.Name)
			total += EstimateTokens(string(call.Arguments))
		}
	}
	return total
}

func ContextSummary(input PromptInput) string {
	coreBytes := 0
	for _, section := range input.CoreDocs {
		coreBytes += len(section.Content)
	}
	return fmt.Sprintf(
		"core_docs=%d, core_bytes=%d, memory_bytes=%d, summary_bytes=%d, tools=%d, skills=%d, history_messages=%d",
		len(input.CoreDocs),
		coreBytes,
		len(input.MemoryContext),
		len(input.SessionSummary),
		len(input.Tools),
		len(input.Skills),
		len(input.RecentHistory),
	)
}

func writeSection(b *strings.Builder, title, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	b.WriteString("\n\n## ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(content))
	b.WriteString("\n")
}
