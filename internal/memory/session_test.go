package memory

import (
	"testing"
	"time"

	"miniclaw-go/internal/core"
)

func TestSessionStoreAppendAndLoad(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	history := []core.Message{
		{Role: "user", Content: "hello", Timestamp: time.Now().UTC()},
		{Role: "assistant", Content: "hi", Timestamp: time.Now().UTC()},
		{Role: "user", Content: "please remember this context", Timestamp: time.Now().UTC()},
	}

	if err := store.Append("demo", history); err != nil {
		t.Fatalf("append: %v", err)
	}
	loaded, err := store.Load("demo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != len(history) {
		t.Fatalf("expected %d messages, got %d", len(history), len(loaded))
	}
}

func TestSessionStoreSetHistoryAndKeepLastMessages(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	history := []core.Message{
		{Role: "user", Content: "one", Timestamp: time.Now().UTC()},
		{Role: "assistant", Content: "two", Timestamp: time.Now().UTC()},
		{Role: "user", Content: "three", Timestamp: time.Now().UTC()},
	}

	if err := store.SetHistory("demo", KeepLastMessages(history, 2)); err != nil {
		t.Fatalf("set history: %v", err)
	}

	loaded, err := store.Load("demo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Content != "two" || loaded[1].Content != "three" {
		t.Fatalf("unexpected history: %+v", loaded)
	}
}
