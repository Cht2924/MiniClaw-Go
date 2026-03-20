package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreLoadsCoreDocsAndSessionMemory(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("agent rules"), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "MEMORY.md"), []byte("memory facts"), 0o644); err != nil {
		t.Fatalf("write MEMORY: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "daily", time.Now().Format("2006-01-02")+".md"), []byte("today note"), 0o644); err != nil {
		t.Fatalf("write daily: %v", err)
	}

	docs, err := store.LoadCoreDocuments()
	if err != nil {
		t.Fatalf("load core docs: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 bootstrap doc, got %d", len(docs))
	}

	memoryContext, err := store.GetMemoryContext(3)
	if err != nil {
		t.Fatalf("get memory context: %v", err)
	}
	if !strings.Contains(memoryContext, "memory facts") {
		t.Fatalf("memory context missing long-term memory: %q", memoryContext)
	}
	if !strings.Contains(memoryContext, "today note") {
		t.Fatalf("memory context missing daily note: %q", memoryContext)
	}

	if err := store.WriteSessionSummary("demo", "# 会话摘要\n\nsummarized history"); err != nil {
		t.Fatalf("write session summary: %v", err)
	}
	summary, err := store.LoadSessionSummary("demo")
	if err != nil {
		t.Fatalf("load session summary: %v", err)
	}
	if !strings.Contains(summary, "summarized history") {
		t.Fatalf("unexpected summary: %q", summary)
	}
}
