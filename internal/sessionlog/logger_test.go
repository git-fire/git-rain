package sessionlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultLogDir(t *testing.T) {
	dir := DefaultLogDir()
	if dir == "" {
		t.Fatal("DefaultLogDir() should not be empty")
	}
	if !strings.Contains(dir, filepath.Join("git-rain", "logs")) {
		t.Fatalf("unexpected DefaultLogDir: %s", dir)
	}
}

func TestLogger_WritesJSONL(t *testing.T) {
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	logger.Info("repo", "scan", "repo discovered")
	logger.Success("repo", "scan-complete", "done", time.Second)
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 log lines, got %d", len(lines))
	}
	for _, line := range lines {
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
	}
}

func TestLogger_Subscribe(t *testing.T) {
	logger, err := NewLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer func() { _ = logger.Close() }()

	seen := make(chan LogEntry, 1)
	logger.Subscribe(func(e LogEntry) { seen <- e })
	logger.Info("repo", "scan", "repo discovered")

	select {
	case entry := <-seen:
		if entry.Action != "scan" {
			t.Fatalf("entry.Action = %q, want scan", entry.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive subscribed log event")
	}
}
