package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"miniclaw-go/internal/core"
)

func TestRunCommandToolExecutesAndTimesOut(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry()
	policy := FilePolicy{
		ProjectRoot: root,
		AllowedRoots: []string{
			root,
		},
	}
	RegisterCommandTool(reg, policy, 1200*time.Millisecond)

	cmd := "Write-Output 'hello'"
	slow := "Start-Sleep -Seconds 2"
	if runtime.GOOS != "windows" {
		cmd = "printf hello"
		slow = "sleep 2"
	}

	call := core.ToolCall{
		ID:   "1",
		Name: "run_command",
		Arguments: mustJSON(t, map[string]string{
			"command": cmd,
			"cwd":     root,
		}),
	}
	result := reg.Execute(context.Background(), call)
	if result.Error != "" {
		t.Fatalf("expected successful command, got error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("unexpected output: %q", result.Content)
	}

	timeoutCall := core.ToolCall{
		ID:   "2",
		Name: "run_command",
		Arguments: mustJSON(t, map[string]string{
			"command": slow,
			"cwd":     root,
		}),
	}
	timeoutResult := reg.Execute(context.Background(), timeoutCall)
	if timeoutResult.Error == "" {
		t.Fatalf("expected timeout error")
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
