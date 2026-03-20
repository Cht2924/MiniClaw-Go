package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"miniclaw-go/internal/core"
)

func RegisterCommandTool(reg *Registry, policy FilePolicy, timeout time.Duration) {
	reg.Register(core.ToolDescriptor{
		Name:        "run_command",
		Description: "Run a shell command within an allowed working directory.",
		Source:      "native",
		InputSchema: schema(
			prop("command", "string", "Shell command to execute."),
			prop("cwd", "string", "Working directory for the command."),
			required("command", "cwd"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Command string `json:"command"`
			CWD     string `json:"cwd"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		cwd, err := resolveAllowedPath(policy, input.CWD, false)
		if err != nil {
			return "", err
		}

		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(cmdCtx, "powershell", "-NoProfile", "-Command", input.Command)
		} else {
			cmd = exec.CommandContext(cmdCtx, "sh", "-c", input.Command)
		}
		cmd.Dir = cwd
		cmd.Env = os.Environ()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		output := strings.TrimSpace(stdout.String())
		errOutput := strings.TrimSpace(stderr.String())
		combined := strings.TrimSpace(strings.Join([]string{output, errOutput}, "\n"))
		combined = truncate(combined, 8000)

		if cmdCtx.Err() == context.DeadlineExceeded {
			return combined, fmt.Errorf("command timed out after %s", timeout)
		}
		if err != nil {
			if combined == "" {
				combined = err.Error()
			}
			return combined, fmt.Errorf("command failed in %s", filepath.ToSlash(cwd))
		}
		if combined == "" {
			combined = "(no output)"
		}
		return combined, nil
	})
}
