package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"miniclaw-go/internal/core"
)

type SessionStore struct {
	root string
}

func NewSessionStore(root string) *SessionStore {
	return &SessionStore{root: root}
}

func (s *SessionStore) Ensure() error {
	return os.MkdirAll(s.root, 0o755)
}

func (s *SessionStore) Append(sessionID string, messages []core.Message) error {
	if err := s.Ensure(); err != nil {
		return err
	}

	path := s.sessionPath(sessionID)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, msg := range messages {
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now().UTC()
		}
		if err := enc.Encode(msg); err != nil {
			return fmt.Errorf("encode message: %w", err)
		}
	}

	return nil
}

func (s *SessionStore) Load(sessionID string) ([]core.Message, error) {
	path := s.sessionPath(sessionID)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer file.Close()

	var messages []core.Message
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg core.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("decode session line: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session file: %w", err)
	}
	return messages, nil
}

func (s *SessionStore) SetHistory(sessionID string, history []core.Message) error {
	if err := s.Ensure(); err != nil {
		return err
	}

	file, err := os.Create(s.sessionPath(sessionID))
	if err != nil {
		return fmt.Errorf("create session file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, msg := range history {
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now().UTC()
		}
		if err := enc.Encode(msg); err != nil {
			return fmt.Errorf("encode session line: %w", err)
		}
	}

	return nil
}

func (s *SessionStore) Stats(sessionID string) (core.SessionStats, error) {
	history, err := s.Load(sessionID)
	if err != nil {
		return core.SessionStats{}, err
	}

	stats := core.SessionStats{
		SessionID:    sessionID,
		HistoryCount: len(history),
	}
	if len(history) > 0 {
		stats.LastActivityAt = history[len(history)-1].Timestamp
	}

	return stats, nil
}

func (s *SessionStore) sessionPath(sessionID string) string {
	return filepath.Join(s.root, sessionID+".jsonl")
}

func KeepLastMessages(history []core.Message, keep int) []core.Message {
	if keep <= 0 {
		return nil
	}
	if len(history) <= keep {
		return append([]core.Message(nil), history...)
	}
	return append([]core.Message(nil), history[len(history)-keep:]...)
}
