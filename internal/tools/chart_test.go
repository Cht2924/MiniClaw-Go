package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"miniclaw-go/internal/core"
)

func TestVisualizeChartToolRendersPNG(t *testing.T) {
	pythonBin, err := findPythonBinary()
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	check := exec.Command(pythonBin, "-c", "import matplotlib")
	if err := check.Run(); err != nil {
		t.Skipf("skip: matplotlib unavailable: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	projectRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	outputRoot := t.TempDir()

	reg := NewRegistry()
	policy := FilePolicy{
		ProjectRoot: projectRoot,
		AllowedRoots: []string{
			projectRoot,
			outputRoot,
		},
	}
	RegisterChartTool(reg, policy)

	outputPath := filepath.Join(outputRoot, "chart.png")
	call := core.ToolCall{
		ID:   "chart-1",
		Name: "visualize_chart",
		Arguments: mustChartJSON(t, chartRequest{
			ChartType:  "bar",
			Title:      "Test Chart",
			Labels:     []string{"A", "B", "C"},
			Values:     []float64{3, 7, 5},
			OutputPath: outputPath,
			XLabel:     "Category",
			YLabel:     "Value",
		}),
	}

	result := reg.Execute(context.Background(), call)
	if result.Error != "" {
		t.Fatalf("expected successful chart render, got error: %s", result.Error)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty png output")
	}
}

func mustChartJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
