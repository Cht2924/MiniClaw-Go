package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"miniclaw-go/internal/core"
)

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Ensure() error {
	return os.MkdirAll(s.root, 0o755)
}

func (s *Store) Write(trace core.RunTrace) (string, error) {
	if err := s.Ensure(); err != nil {
		return "", err
	}
	dir := filepath.Join(s.root, trace.SessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, trace.RunID+".json")
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal trace: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write trace: %w", err)
	}
	return path, nil
}
