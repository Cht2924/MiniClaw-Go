package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"miniclaw-go/internal/core"
	"miniclaw-go/internal/llm"
	"miniclaw-go/internal/tools"
)

type ChatModel interface {
	Complete(ctx context.Context, systemPrompt string, history []core.Message, tools []core.ToolDescriptor) (llm.Completion, error)
}

type Loop struct {
	model      ChatModel
	registry   *tools.Registry
	maxSteps   int
	skillsRoot string
}

func NewLoop(model ChatModel, registry *tools.Registry, maxSteps int, skillsRoot string) *Loop {
	return &Loop{
		model:      model,
		registry:   registry,
		maxSteps:   maxSteps,
		skillsRoot: filepath.Clean(skillsRoot),
	}
}

func (l *Loop) Run(ctx context.Context, runCtx core.RunContext, systemPrompt string, history []core.Message, toolDescs []core.ToolDescriptor) ([]core.Message, core.RunTrace, core.RunResult, error) {
	trace := core.RunTrace{
		SessionID: runCtx.SessionID,
		RunID:     runCtx.RunID,
		StartedAt: runCtx.StartTime,
		UserInput: runCtx.UserInput,
		Tools:     descriptorNames(toolDescs),
	}

	messages := append([]core.Message(nil), history...)
	userMsg := core.Message{
		ID:        core.NewID("msg"),
		Role:      "user",
		Content:   runCtx.UserInput,
		Timestamp: time.Now().UTC(),
	}
	messages = append(messages, userMsg)
	newMessages := []core.Message{userMsg}

	usedTools := map[string]struct{}{}
	usedSkills := map[string]struct{}{}

	for step := 1; step <= l.maxSteps; step++ {
		var (
			completion llm.Completion
			err        error
		)
		activeSystemPrompt := systemPrompt
		for retry := 0; retry < 3; retry++ {
			completion, err = l.model.Complete(ctx, activeSystemPrompt, messages, toolDescs)
			if err == nil {
				systemPrompt = activeSystemPrompt
				break
			}
			if !llm.IsContextWindowError(err) {
				break
			}
			var dropped int
			activeSystemPrompt, messages, dropped = forceCompressHistory(activeSystemPrompt, messages)
			if dropped == 0 {
				break
			}
			trace.Metadata = ensureTraceMetadata(trace.Metadata)
			trace.Metadata["emergency_compressions"] = toInt(trace.Metadata["emergency_compressions"]) + 1
		}
		if err != nil {
			trace.StopReason = "model_error"
			return newMessages, trace, core.RunResult{}, err
		}

		if len(completion.ToolCalls) == 0 {
			msg := core.Message{
				ID:        core.NewID("msg"),
				Role:      "assistant",
				Content:   strings.TrimSpace(completion.Content),
				Timestamp: time.Now().UTC(),
			}
			messages = append(messages, msg)
			newMessages = append(newMessages, msg)
			trace.Steps = append(trace.Steps, core.TraceStep{
				Index:         step,
				ModelDecision: "final_answer",
				AssistantText: msg.Content,
			})
			trace.StopReason = "final_answer"
			result := core.RunResult{
				Output:     msg.Content,
				UsedTools:  setToList(usedTools),
				UsedSkills: setToList(usedSkills),
			}
			return newMessages, trace, result, nil
		}

		call := completion.ToolCalls[0]
		if call.ID == "" {
			call.ID = core.NewID("toolcall")
		}
		assistantMsg := core.Message{
			ID:        core.NewID("msg"),
			Role:      "assistant",
			Content:   strings.TrimSpace(completion.Content),
			ToolCalls: []core.ToolCall{call},
			Timestamp: time.Now().UTC(),
		}
		messages = append(messages, assistantMsg)
		newMessages = append(newMessages, assistantMsg)

		result := l.registry.Execute(ctx, call)
		toolMsg := core.Message{
			ID:         core.NewID("msg"),
			Role:       "tool",
			Content:    firstNonEmpty(result.Content, result.Error),
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Timestamp:  time.Now().UTC(),
			Meta: map[string]any{
				"duration_ms": result.DurationMS,
				"error":       result.Error,
			},
		}
		messages = append(messages, toolMsg)
		newMessages = append(newMessages, toolMsg)

		trace.Steps = append(trace.Steps, core.TraceStep{
			Index:         step,
			ModelDecision: "tool_call",
			AssistantText: assistantMsg.Content,
			ToolCall:      &call,
			ToolResult:    &result,
		})

		usedTools[call.Name] = struct{}{}
		l.captureSkillUsage(call, result, usedSkills)

		if result.Error != "" {
			trace.StopReason = "tool_error"
			msg := core.Message{
				ID:        core.NewID("msg"),
				Role:      "assistant",
				Content:   fmt.Sprintf("工具 %s 执行失败：%s", call.Name, result.Error),
				Timestamp: time.Now().UTC(),
			}
			newMessages = append(newMessages, msg)
			trace.Steps = append(trace.Steps, core.TraceStep{
				Index:         step,
				ModelDecision: "final_answer",
				AssistantText: msg.Content,
			})
			return newMessages, trace, core.RunResult{
				Output:     msg.Content,
				UsedTools:  setToList(usedTools),
				UsedSkills: setToList(usedSkills),
			}, nil
		}
	}

	trace.StopReason = "max_steps"
	msg := core.Message{
		ID:        core.NewID("msg"),
		Role:      "assistant",
		Content:   "我在完成任务前触达了步骤上限。你可以缩小任务范围，或者让我基于当前结果继续。",
		Timestamp: time.Now().UTC(),
	}
	newMessages = append(newMessages, msg)
	return newMessages, trace, core.RunResult{
		Output:     msg.Content,
		UsedTools:  setToList(usedTools),
		UsedSkills: setToList(usedSkills),
	}, nil
}

func forceCompressHistory(systemPrompt string, history []core.Message) (string, []core.Message, int) {
	if len(history) <= 4 {
		return systemPrompt, history, 0
	}

	conversation := history[:len(history)-1]
	if len(conversation) == 0 {
		return systemPrompt, history, 0
	}

	mid := len(conversation) / 2
	dropped := mid
	kept := append([]core.Message(nil), conversation[mid:]...)
	kept = append(kept, history[len(history)-1])
	kept = dropInvalidCompressedPrefix(kept)
	if len(kept) == 0 {
		kept = []core.Message{history[len(history)-1]}
	}

	note := fmt.Sprintf("\n\n[系统说明：由于上下文超限，已紧急压缩并丢弃最早的 %d 条消息。]", dropped)
	if !strings.Contains(systemPrompt, note) {
		systemPrompt += note
	}
	return systemPrompt, kept, dropped
}

func dropInvalidCompressedPrefix(history []core.Message) []core.Message {
	for len(history) > 0 && history[0].Role == "tool" {
		history = history[1:]
	}
	for len(history) > 0 && history[0].Role == "assistant" && len(history[0].ToolCalls) > 0 {
		history = history[1:]
		for len(history) > 0 && history[0].Role == "tool" {
			history = history[1:]
		}
	}
	return history
}

func ensureTraceMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	return meta
}

func toInt(value any) int {
	v, ok := value.(int)
	if ok {
		return v
	}
	return 0
}

func (l *Loop) captureSkillUsage(call core.ToolCall, result core.ToolResult, used map[string]struct{}) {
	if call.Name != "read_file" {
		return
	}
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(call.Arguments, &payload); err != nil {
		return
	}
	clean := filepath.ToSlash(filepath.Clean(payload.Path))
	if strings.Contains(clean, filepath.ToSlash(l.skillsRoot)) || strings.Contains(strings.ToLower(clean), "/skills/") {
		name := filepath.Base(filepath.Dir(clean))
		if name != "." && name != "" {
			used[name] = struct{}{}
		}
	}
	if strings.Contains(strings.ToLower(result.Content), "## tool preference") {
		name := filepath.Base(filepath.Dir(clean))
		if name != "." && name != "" {
			used[name] = struct{}{}
		}
	}
}

func descriptorNames(descs []core.ToolDescriptor) []string {
	out := make([]string, 0, len(descs))
	for _, desc := range descs {
		out = append(out, desc.Name)
	}
	return out
}

func setToList(set map[string]struct{}) []string {
	var out []string
	for item := range set {
		out = append(out, item)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
