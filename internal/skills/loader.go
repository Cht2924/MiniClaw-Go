package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"miniclaw-go/internal/core"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	root string
}

func NewLoader(root string) *Loader {
	return &Loader{root: root}
}

func (l *Loader) Load() ([]core.SkillDescriptor, error) {
	if err := os.MkdirAll(l.root, 0o755); err != nil {
		return nil, err
	}

	var skills []core.SkillDescriptor
	err := filepath.WalkDir(l.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}

		desc, err := parseSkill(path, l.root)
		if err != nil {
			return err
		}
		skills = append(skills, desc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan skills: %w", err)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func parseSkill(path, root string) (core.SkillDescriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return core.SkillDescriptor{}, fmt.Errorf("read skill: %w", err)
	}
	content := string(data)

	var name, useWhen string

	// Try to parse YAML frontmatter
	if strings.HasPrefix(content, "---") {
		name, useWhen, content = parseFrontMatter(content)
	}

	// Fallback to old parsing if no frontmatter
	if name == "" {
		lines := strings.Split(content, "\n")
		name = strings.TrimSpace(strings.TrimPrefix(firstHeading(lines), "#"))
		if name == "" {
			name = filepath.Base(filepath.Dir(path))
		}
	}

	if useWhen == "" {
		lines := strings.Split(content, "\n")
		useWhen = sectionBody(lines, "Use When")
		if useWhen == "" {
			useWhen = "Use when this skill helps complete the task."
		}
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return core.SkillDescriptor{}, err
	}

	return core.SkillDescriptor{
		Name:     name,
		Path:     filepath.ToSlash(rel),
		UseWhen:  compact(useWhen),
		Summary:  compact(useWhen),
		Contents: content,
	}, nil
}

// parseFrontMatter extracts name and description from YAML frontmatter
// and returns the remaining content after the closing ---
func parseFrontMatter(content string) (name, description, body string) {
	lines := strings.Split(content, "\n")

	// Find the closing ---
	endIdx := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			endIdx = i + 1
			break
		}
	}

	if endIdx <= 0 {
		return "", "", content
	}

	// Parse the YAML between --- and ---
	frontMatter := strings.Join(lines[1:endIdx], "\n")

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontMatter), &fm); err != nil {
		return "", "", content
	}

	if n, ok := fm["name"].(string); ok {
		name = n
	}
	if d, ok := fm["description"].(string); ok {
		description = d
	}

	// Return remaining content after the closing ---
	body = strings.Join(lines[endIdx+1:], "\n")
	return
}

func firstHeading(lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func sectionBody(lines []string, title string) string {
	var body []string
	inSection := false
	needle := "## " + strings.ToLower(title)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "## "):
			if lower == needle {
				inSection = true
				continue
			}
			if inSection {
				return strings.TrimSpace(strings.Join(body, "\n"))
			}
		default:
			if inSection {
				body = append(body, line)
			}
		}
	}
	return strings.TrimSpace(strings.Join(body, "\n"))
}

func compact(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
