package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"

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
