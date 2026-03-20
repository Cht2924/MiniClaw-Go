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

	"miniclaw-go/internal/core"
)

type chartRequest struct {
	ChartType  string    `json:"chart_type"`
	Title      string    `json:"title"`
	Labels     []string  `json:"labels"`
	Values     []float64 `json:"values"`
	OutputPath string    `json:"output_path"`
	XLabel     string    `json:"x_label,omitempty"`
	YLabel     string    `json:"y_label,omitempty"`
}

func RegisterChartTool(reg *Registry, policy FilePolicy) {
	reg.Register(core.ToolDescriptor{
		Name:        "visualize_chart",
		Description: "Render a bar chart or pie chart with Python/matplotlib and save it as a PNG file.",
		Source:      "native",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chart_type": map[string]any{
					"type":        "string",
					"description": "Chart type. Supported values: bar, pie.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Chart title.",
				},
				"labels": map[string]any{
					"type":        "array",
					"description": "Labels for data points.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"values": map[string]any{
					"type":        "array",
					"description": "Numeric values for data points.",
					"items": map[string]any{
						"type": "number",
					},
				},
				"output_path": map[string]any{
					"type":        "string",
					"description": "PNG output path within allowed roots.",
				},
				"x_label": map[string]any{
					"type":        "string",
					"description": "Optional x-axis label for bar charts.",
				},
				"y_label": map[string]any{
					"type":        "string",
					"description": "Optional y-axis label for bar charts.",
				},
			},
			"required": []string{"chart_type", "title", "labels", "values", "output_path"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input chartRequest
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		if err := validateChartRequest(input); err != nil {
			return "", err
		}

		outputPath, err := resolveAllowedPath(policy, input.OutputPath, true)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return "", err
		}
		input.OutputPath = outputPath

		scriptPath := filepath.Join(policy.ProjectRoot, "internal", "tools", "scripts", "render_chart.py")
		if _, err := os.Stat(scriptPath); err != nil {
			return "", fmt.Errorf("chart renderer script missing: %w", err)
		}

		pythonBin, err := findPythonBinary()
		if err != nil {
			return "", err
		}

		payloadFile, err := os.CreateTemp("", "miniclaw-chart-*.json")
		if err != nil {
			return "", err
		}
		payloadPath := payloadFile.Name()
		defer os.Remove(payloadPath)
		defer payloadFile.Close()

		encoder := json.NewEncoder(payloadFile)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(input); err != nil {
			return "", err
		}

		cmd := exec.CommandContext(ctx, pythonBin, scriptPath, "--input", payloadPath)
		cmd.Dir = policy.ProjectRoot
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errText := strings.TrimSpace(stderr.String())
			if errText == "" {
				errText = strings.TrimSpace(stdout.String())
			}
			if errText != "" {
				return "", fmt.Errorf("render chart: %w: %s", err, truncate(errText, 2000))
			}
			return "", fmt.Errorf("render chart: %w", err)
		}

		rel := input.OutputPath
		if relPath, err := filepath.Rel(policy.ProjectRoot, input.OutputPath); err == nil {
			rel = filepath.ToSlash(relPath)
		}
		return fmt.Sprintf("rendered %s chart to %s", input.ChartType, rel), nil
	})
}

func validateChartRequest(input chartRequest) error {
	input.ChartType = strings.TrimSpace(strings.ToLower(input.ChartType))
	if input.ChartType != "bar" && input.ChartType != "pie" {
		return fmt.Errorf("chart_type must be 'bar' or 'pie'")
	}
	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output_path is required")
	}
	if len(input.Labels) == 0 || len(input.Values) == 0 {
		return fmt.Errorf("labels and values are required")
	}
	if len(input.Labels) != len(input.Values) {
		return fmt.Errorf("labels and values must have the same length")
	}

	total := 0.0
	for i, label := range input.Labels {
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("labels[%d] is empty", i)
		}
		if input.Values[i] < 0 {
			return fmt.Errorf("values[%d] must be >= 0", i)
		}
		total += input.Values[i]
	}
	if input.ChartType == "pie" && total <= 0 {
		return fmt.Errorf("pie chart values must sum to > 0")
	}
	return nil
}

func findPythonBinary() (string, error) {
	candidates := []string{"python"}
	if runtime.GOOS != "windows" {
		candidates = append([]string{"python3"}, candidates...)
	}
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python interpreter not found in PATH")
}
