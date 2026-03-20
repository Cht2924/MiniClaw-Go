package agent

import (
	"strings"
	"testing"

	"miniclaw-go/internal/core"
)

func TestBuildSystemPromptIncludesOrderedSections(t *testing.T) {
	input := PromptInput{
		CoreDocs: []PromptSection{
			{Title: "AGENTS", Content: "agents block"},
			{Title: "SOUL", Content: "soul block"},
		},
		MemoryContext:  "memory block",
		SessionSummary: "summary block",
		Tools: []core.ToolDescriptor{
			{Name: "read_file", Source: "native", Description: "read a file"},
		},
		Skills: []core.SkillDescriptor{
			{Name: "File Writer Skill", Path: "file_writer/SKILL.md", UseWhen: "Use when writing files"},
		},
	}

	prompt := BuildSystemPrompt(input)
	agentsIndex := strings.Index(prompt, "## AGENTS")
	memoryIndex := strings.Index(prompt, "## 记忆上下文")
	summaryIndex := strings.Index(prompt, "## 会话摘要")
	toolIndex := strings.Index(prompt, "## 工具列表")
	skillIndex := strings.Index(prompt, "## 技能列表")

	if !(agentsIndex < memoryIndex && memoryIndex < summaryIndex && summaryIndex < toolIndex && toolIndex < skillIndex) {
		t.Fatalf("sections out of order: %q", prompt)
	}
}
