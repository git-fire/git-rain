package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"

	"github.com/git-rain/git-rain/internal/rainexec"
)

func TestRainRunModel_AppliesProgressEvents(t *testing.T) {
	m := rainRunModel{
		rows: []rainRepoRow{{path: "/a", name: "alpha", state: rainexec.StateQueued}},
	}
	started := time.Now()
	m.applyEvent(rainexec.Event{
		RepoPath: "/a",
		State:    rainexec.StateRunning,
		StartedAt: started,
		Attempt:  1,
		MaxAttempts: 2,
	})
	if m.rows[0].state != rainexec.StateRunning {
		t.Fatalf("state = %q", m.rows[0].state)
	}
	m.applyEvent(rainexec.Event{RepoPath: "/a", State: rainexec.StateRetrying, Attempt: 2, MaxAttempts: 2})
	if m.rows[0].state != rainexec.StateRetrying {
		t.Fatalf("state = %q", m.rows[0].state)
	}
}

func TestRainRunModel_SkipKeyInvokesControl(t *testing.T) {
	skipped := ""
	m := rainRunModel{
		rows: []rainRepoRow{{path: "/a", name: "alpha", state: rainexec.StateRunning}},
		ctrl: RainControls{Skip: func(p string) { skipped = p }},
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if skipped != "/a" {
		t.Fatalf("skip path = %q", skipped)
	}
}

func TestRainRunModel_AbortKeySetsFlag(t *testing.T) {
	aborted := false
	m := rainRunModel{
		rows: []rainRepoRow{{path: "/a", name: "alpha"}},
		ctrl: RainControls{Abort: func() { aborted = true }},
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	fm := next.(rainRunModel)
	if !fm.aborted || !aborted {
		t.Fatalf("expected abort")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestRainRunModel_QuitKeyAborts(t *testing.T) {
	aborted := false
	m := rainRunModel{
		rows: []rainRepoRow{{path: "/a", name: "alpha", state: rainexec.StateRunning}},
		ctrl: RainControls{Abort: func() { aborted = true }},
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	fm := next.(rainRunModel)
	if !fm.aborted || !aborted {
		t.Fatal("expected q to abort the rain run")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestRainRunModel_OverdueHighlight(t *testing.T) {
	m := rainRunModel{
		timeoutSec: 2,
		width:      80,
	}
	onTime := rainRepoRow{
		name:      "alpha",
		state:     rainexec.StateRunning,
		startedAt: time.Now().Add(-1 * time.Second),
	}
	overdue := rainRepoRow{
		name:      "alpha",
		state:     rainexec.StateRunning,
		startedAt: time.Now().Add(-5 * time.Second),
	}
	if m.formatRow(onTime, false) == m.formatRow(overdue, false) {
		t.Fatal("expected overdue row to differ from on-time row")
	}
}

func TestRainRunModel_ViewScrollsToFollowCursor(t *testing.T) {
	rows := make([]rainRepoRow, 25)
	for i := range rows {
		rows[i] = rainRepoRow{
			path: fmt.Sprintf("/repo/%d", i),
			name: fmt.Sprintf("repo-%02d", i),
			state: rainexec.StateQueued,
		}
	}
	m := rainRunModel{
		rows:   rows,
		cursor: 20,
		height: 10,
		width:  80,
	}
	m.scrollOffset = m.clampRainScroll()
	view := m.View()
	if !strings.Contains(view, "repo-20") {
		t.Fatalf("expected cursor repo in view, got:\n%s", view)
	}
	if strings.Contains(view, "repo-00\n") || strings.Contains(view, "  repo-00") {
		t.Fatalf("first repo should be off-screen when cursor is near the end")
	}

	m = rainRunModel{rows: rows, cursor: 0, height: 10, width: 80}
	m.scrollOffset = m.clampRainScroll()
	view = m.View()
	if !strings.Contains(view, "repo-00") {
		t.Fatalf("expected first repo visible at top cursor")
	}

	for i := 0; i < 18; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(rainRunModel)
	}
	view = m.View()
	if m.cursor != 18 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	if !strings.Contains(view, "repo-18") {
		t.Fatalf("expected repo-18 visible after scrolling down, got:\n%s", view)
	}
}

func TestRainRunModel_OverdueTruncateKeepsReset(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := rainRunModel{
		timeoutSec: 1,
		width:      40,
	}
	row := rainRepoRow{
		name:      strings.Repeat("long-repo-name-", 8),
		state:     rainexec.StateRunning,
		startedAt: time.Now().Add(-10 * time.Second),
	}
	got := m.formatRow(row, false)
	wantWidth := max(40, m.width-2)
	if lipgloss.Width(got) != wantWidth {
		t.Fatalf("lipgloss.Width = %d, want %d (ANSI reset likely clipped)", lipgloss.Width(got), wantWidth)
	}
	// Style-then-truncate would clip the reset; truncate-then-style keeps it.
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("expected intact lipgloss reset suffix, got %q", got)
	}
	sec := int(time.Since(row.startedAt).Seconds())
	plain := runewidth.Truncate("  "+row.name+fmt.Sprintf(" — running %ds", sec), wantWidth, "…")
	want := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(plain)
	if got != want {
		t.Fatalf("formatRow = %q, want %q", got, want)
	}
}
