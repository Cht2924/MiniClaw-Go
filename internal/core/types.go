package core

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

type Message struct {
	ID         string         `json:"id,omitempty"`
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Meta       map[string]any `json:"meta,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name"`
	Content    string `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Source      string         `json:"source"`
}

type SkillDescriptor struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	UseWhen  string `json:"use_when"`
	Summary  string `json:"summary"`
	Contents string `json:"-"`
}

type RunContext struct {
	SessionID string    `json:"session_id"`
	RunID     string    `json:"run_id"`
	UserInput string    `json:"user_input"`
	StartTime time.Time `json:"start_time"`
}

type TraceStep struct {
	Index         int         `json:"index"`
	ModelDecision string      `json:"model_decision"`
	AssistantText string      `json:"assistant_text,omitempty"`
	ToolCall      *ToolCall   `json:"tool_call,omitempty"`
	ToolResult    *ToolResult `json:"tool_result,omitempty"`
}

type RunTrace struct {
	SessionID      string         `json:"session_id"`
	RunID          string         `json:"run_id"`
	StartedAt      time.Time      `json:"started_at"`
	UserInput      string         `json:"user_input"`
	ContextSummary string         `json:"context_summary"`
	Tools          []string       `json:"tools"`
	Skills         []string       `json:"skills"`
	Steps          []TraceStep    `json:"steps"`
	StopReason     string         `json:"stop_reason"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type RunResult struct {
	Output     string   `json:"output"`
	UsedTools  []string `json:"used_tools"`
	UsedSkills []string `json:"used_skills"`
	TracePath  string   `json:"trace_path"`
}

type SessionStats struct {
	SessionID      string    `json:"session_id"`
	HistoryCount   int       `json:"history_count"`
	SummaryExists  bool      `json:"summary_exists"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

var idCounter uint64

func NewID(prefix string) string {
	ts := time.Now().UTC().Format("20060102T150405.000")
	seq := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%s_%s_%04d", prefix, ts, seq)
}
