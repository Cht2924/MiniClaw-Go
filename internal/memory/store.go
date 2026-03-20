package memory

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var coreMemoryDocs = []string{
	"AGENTS.md",
	"SOUL.md",
	"IDENTITY.md",
	"USER.md",
	"TOOLS.md",
}

type Document struct {
	Name    string
	Title   string
	Content string
}

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Ensure() error {
	dirs := []string{
		s.root,
		filepath.Join(s.root, "daily"),
		filepath.Join(s.root, "session"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListFiles() ([]string, error) {
	if err := s.Ensure(); err != nil {
		return nil, err
	}
	var files []string
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			rel, err := filepath.Rel(s.root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan memory: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) LoadCoreDocuments() ([]Document, error) {
	if err := s.Ensure(); err != nil {
		return nil, err
	}

	var docs []Document
	for _, name := range coreMemoryDocs {
		content, err := s.readOptional(name)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		docs = append(docs, Document{
			Name:    name,
			Title:   strings.TrimSuffix(name, filepath.Ext(name)),
			Content: strings.TrimSpace(content),
		})
	}
	return docs, nil
}

func (s *Store) GetRecentDailyNotes(days int) (string, error) {
	if err := s.Ensure(); err != nil {
		return "", err
	}
	if days <= 0 {
		return "", nil
	}

	var parts []string
	for i := 0; i < days; i++ {
		note, err := s.readOptional(filepath.Join("daily", time.Now().AddDate(0, 0, -i).Format("2006-01-02")+".md"))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(note) != "" {
			parts = append(parts, strings.TrimSpace(note))
		}
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

func (s *Store) GetMemoryContext(days int) (string, error) {
	longTerm, err := s.readOptional("MEMORY.md")
	if err != nil {
		return "", err
	}
	recentNotes, err := s.GetRecentDailyNotes(days)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(longTerm) == "" && strings.TrimSpace(recentNotes) == "" {
		return "", nil
	}

	var b strings.Builder
	if strings.TrimSpace(longTerm) != "" {
		b.WriteString("## 长期记忆\n\n")
		b.WriteString(strings.TrimSpace(longTerm))
	}
	if strings.TrimSpace(recentNotes) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n---\n\n")
		}
		b.WriteString("## 最近日记\n\n")
		b.WriteString(strings.TrimSpace(recentNotes))
	}
	return b.String(), nil
}

func (s *Store) LoadSessionSummary(sessionID string) (string, error) {
	return s.readOptional(filepath.Join("session", sessionID, "SUMMARY.md"))
}

func (s *Store) SessionSummaryExists(sessionID string) (bool, error) {
	path := filepath.Join(s.root, "session", sessionID, "SUMMARY.md")
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *Store) AppendLongTerm(filename, content string) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	path := filepath.Join(s.root, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open memory file: %w", err)
	}
	defer file.Close()

	entry := fmt.Sprintf("\n- %s %s\n", time.Now().Format(time.RFC3339), strings.TrimSpace(content))
	_, err = file.WriteString(entry)
	return err
}

func (s *Store) WriteSessionSummary(sessionID, summary string) error {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return s.writeSessionFile(sessionID, "SUMMARY.md", "")
	}
	return s.writeSessionFile(sessionID, "SUMMARY.md", summary+"\n")
}

func (s *Store) readOptional(rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.root, rel))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read memory %s: %w", rel, err)
	}
	return string(data), nil
}

func (s *Store) writeSessionFile(sessionID, filename, content string) error {
	path := filepath.Join(s.root, "session", sessionID, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
