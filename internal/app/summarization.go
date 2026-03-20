package app

import (
	"context"
	"fmt"
	"strings"

	"miniclaw-go/internal/agent"
	"miniclaw-go/internal/core"
	"miniclaw-go/internal/memory"
)

const maxSummarizationMessages = 10

func (a *App) maybeCompressSession(ctx context.Context, sessionID, existingSummary string, history []core.Message) ([]core.Message, bool, error) {
	keepRecent := a.cfg.Agent.SummaryKeepRecent
	if keepRecent <= 0 {
		keepRecent = 4
	}
	if len(history) <= keepRecent {
		return history, false, nil
	}

	messageThreshold := a.cfg.Agent.SummarizeMessageThreshold
	tokenThreshold := a.cfg.LLM.MaxContextTokens * a.cfg.Agent.SummarizeTokenPercent / 100
	historyTokens := agent.EstimateHistoryTokens(history)
	if (messageThreshold <= 0 || len(history) <= messageThreshold) && (tokenThreshold <= 0 || historyTokens <= tokenThreshold) {
		return history, false, nil
	}

	summary, err := a.summarizeSessionHistory(ctx, existingSummary, history, keepRecent)
	if err != nil {
		return history, false, err
	}
	if strings.TrimSpace(summary) == "" {
		return history, false, nil
	}

	if err := a.memoryStore.WriteSessionSummary(sessionID, summary); err != nil {
		return history, false, err
	}

	kept := memory.KeepLastMessages(history, keepRecent)
	if err := a.sessionStore.SetHistory(sessionID, kept); err != nil {
		return history, false, err
	}

	return kept, true, nil
}

func (a *App) summarizeSessionHistory(ctx context.Context, existingSummary string, history []core.Message, keepRecent int) (string, error) {
	if len(history) <= keepRecent {
		return "", nil
	}

	toSummarize := history[:len(history)-keepRecent]
	maxMessageTokens := maxInt(1, a.cfg.LLM.MaxContextTokens/2)

	validMessages := make([]core.Message, 0, len(toSummarize))
	omitted := false
	for _, msg := range toSummarize {
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if agent.EstimateTokens(msg.Content) > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, msg)
	}
	if len(validMessages) == 0 {
		return "", nil
	}

	var summary string
	if len(validMessages) > maxSummarizationMessages {
		mid := findNearestUserMessage(validMessages, len(validMessages)/2)
		if mid <= 0 || mid >= len(validMessages) {
			mid = len(validMessages) / 2
		}

		first, err := a.summarizeBatch(ctx, validMessages[:mid], "")
		if err != nil {
			return "", err
		}
		second, err := a.summarizeBatch(ctx, validMessages[mid:], "")
		if err != nil {
			return "", err
		}

		mergePrompt := fmt.Sprintf(
			"请将下面两段会话摘要合并为一份连贯、去重、便于继续工作的摘要。\n\n摘要 1：\n%s\n\n摘要 2：\n%s",
			first,
			second,
		)
		merged, err := a.llmClient.CompleteText(ctx, summarySystemPrompt(), []core.Message{{
			Role:    "user",
			Content: mergePrompt,
		}})
		if err == nil && strings.TrimSpace(merged) != "" {
			summary = strings.TrimSpace(merged)
		} else {
			summary = strings.TrimSpace(first + "\n\n" + second)
		}
	} else {
		var err error
		summary, err = a.summarizeBatch(ctx, validMessages, existingSummary)
		if err != nil {
			return "", err
		}
	}

	if omitted && strings.TrimSpace(summary) != "" {
		summary += "\n\n[说明：为控制上下文长度，部分超长消息未被纳入摘要。]"
	}
	if strings.TrimSpace(summary) == "" {
		return "", nil
	}
	return "# 会话摘要\n\n" + strings.TrimSpace(summary), nil
}

func (a *App) summarizeBatch(ctx context.Context, batch []core.Message, existingSummary string) (string, error) {
	var b strings.Builder
	b.WriteString("请对下面这段会话做简洁摘要，并保留继续工作需要的关键信息：用户目标、限制条件、已做决策、涉及文件、工具结果、未完成事项与下一步。\n")
	if strings.TrimSpace(existingSummary) != "" {
		b.WriteString("\n已有摘要：\n")
		b.WriteString(strings.TrimSpace(existingSummary))
		b.WriteString("\n")
	}
	b.WriteString("\n会话内容：\n")
	for _, msg := range batch {
		b.WriteString(msg.Role)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(msg.Content))
		b.WriteString("\n")
	}

	response, err := a.llmClient.CompleteText(ctx, summarySystemPrompt(), []core.Message{{
		Role:    "user",
		Content: b.String(),
	}})
	if err == nil && strings.TrimSpace(response) != "" {
		return strings.TrimSpace(response), nil
	}

	return fallbackSummary(batch), nil
}

func summarySystemPrompt() string {
	return "你负责为本地 CLI Agent 压缩历史会话。请保持简洁、客观、面向后续执行，不要编造事实。"
}

func fallbackSummary(batch []core.Message) string {
	var b strings.Builder
	b.WriteString("会话摘要：")
	for i, msg := range batch {
		if i > 0 {
			b.WriteString(" | ")
		}
		content := strings.TrimSpace(msg.Content)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		b.WriteString(msg.Role)
		b.WriteString(": ")
		b.WriteString(content)
	}
	return b.String()
}

func findNearestUserMessage(messages []core.Message, mid int) int {
	originalMid := mid
	for mid > 0 && messages[mid].Role != "user" {
		mid--
	}
	if messages[mid].Role == "user" {
		return mid
	}

	mid = originalMid
	for mid < len(messages) && messages[mid].Role != "user" {
		mid++
	}
	if mid < len(messages) {
		return mid
	}
	return originalMid
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
