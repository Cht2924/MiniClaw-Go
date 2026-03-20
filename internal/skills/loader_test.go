package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderParsesSkillNameAndUseWhen(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skill := `# Demo Skill

## Use When

Use this skill to test parsing.

## Steps

1. Do a thing.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	loader := NewLoader(root)
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "Demo Skill" {
		t.Fatalf("unexpected skill name: %s", skills[0].Name)
	}
	if skills[0].UseWhen != "Use this skill to test parsing." {
		t.Fatalf("unexpected use-when: %s", skills[0].UseWhen)
	}
}

func TestLoaderParsesFrontMatter(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "tmux")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skill := `---
name: tmux
description: Remote-control tmux sessions for interactive CLIs.
---

# tmux Skill

Use tmux only when you need an interactive TTY.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	loader := NewLoader(root)
	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "tmux" {
		t.Fatalf("unexpected skill name: %s", skills[0].Name)
	}
	if skills[0].UseWhen != "Remote-control tmux sessions for interactive CLIs." {
		t.Fatalf("unexpected use-when: %s", skills[0].UseWhen)
	}
}
