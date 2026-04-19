package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/git-rain/git-rain/internal/sessionlog"
)

func TestStatusGlyphFromEntry(t *testing.T) {
	if got := statusGlyph(sessionlog.LogEntry{Level: "success", Action: "scan-complete"}); got != "✅" {
		t.Fatalf("statusGlyph(success) = %q, want ✅", got)
	}
	if got := statusGlyph(sessionlog.LogEntry{Level: "error", Action: "scan-failed"}); got != "❌" {
		t.Fatalf("statusGlyph(error) = %q, want ❌", got)
	}
}

func TestRenderLogExportText(t *testing.T) {
	out := renderLogExportText([]sessionlog.LogEntry{
		{
			Timestamp:   time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
			Level:       "info",
			Action:      "scan-progress",
			Description: "found repo",
		},
	})
	if !strings.Contains(out, "scan-progress") {
		t.Fatalf("missing action in output: %q", out)
	}
}

func TestRepoSelectorModel_ToggleLogPanel(t *testing.T) {
	m := NewRepoSelectorModel(nil, nil, "")
	if m.showLogPanel {
		t.Fatal("showLogPanel should start false")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	next := updated.(RepoSelectorModel)
	if !next.showLogPanel {
		t.Fatal("showLogPanel should be true after pressing Shift+L")
	}
}
