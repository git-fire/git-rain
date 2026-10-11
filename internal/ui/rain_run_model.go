package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/rainexec"
)

// ErrRainAborted is returned when the user aborts the rain phase from the TUI.
var ErrRainAborted = fmt.Errorf("rain aborted")

// RainControls exposes skip/abort actions for the rain-phase TUI.
type RainControls struct {
	Skip  func(repoPath string)
	Abort func()
}

type rainRepoRow struct {
	path, name string
	state      rainexec.RepoState
	attempt    int
	maxAttempts int
	startedAt  time.Time
	detail     string
}

type rainRunModel struct {
	rows       []rainRepoRow
	cursor     int
	timeoutSec int
	events     <-chan rainexec.Event
	ctrl       RainControls
	aborted    bool
	width      int
	height     int
}

type rainEventMsg rainexec.Event

type rainTickMsg time.Time

// RunRainProgressTUI shows live per-repo rain status until events is closed.
func RunRainProgressTUI(repos []git.Repository, events <-chan rainexec.Event, ctrl RainControls, timeoutSec int) error {
	rows := make([]rainRepoRow, len(repos))
	for i, r := range repos {
		rows[i] = rainRepoRow{path: r.Path, name: r.Name, state: rainexec.StateQueued}
	}
	m := rainRunModel{
		rows:       rows,
		timeoutSec: timeoutSec,
		events:     events,
		ctrl:       ctrl,
	}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if fm, ok := final.(rainRunModel); ok && fm.aborted {
		return ErrRainAborted
	}
	return nil
}

func (m rainRunModel) Init() tea.Cmd {
	return tea.Batch(listenRainEvent(m.events), tickRain())
}

func listenRainEvent(ch <-chan rainexec.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return rainEventMsg(e)
	}
}

func tickRain() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return rainTickMsg(t) })
}

func (m rainRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case rainEventMsg:
		m.applyEvent(rainexec.Event(msg))
		return m, listenRainEvent(m.events)
	case rainTickMsg:
		return m, tickRain()
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "s":
			if m.ctrl.Skip != nil && m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				if row.state == rainexec.StateRunning || row.state == rainexec.StateRetrying {
					m.ctrl.Skip(row.path)
				}
			}
		case "a", "q", "ctrl+c":
			m.aborted = true
			if m.ctrl.Abort != nil {
				m.ctrl.Abort()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *rainRunModel) applyEvent(e rainexec.Event) {
	for i := range m.rows {
		if m.rows[i].path == e.RepoPath {
			m.rows[i].state = e.State
			m.rows[i].attempt = e.Attempt
			m.rows[i].maxAttempts = e.MaxAttempts
			if !e.StartedAt.IsZero() {
				m.rows[i].startedAt = e.StartedAt
			}
			if e.Detail != "" {
				m.rows[i].detail = e.Detail
			}
			return
		}
	}
}

func (m rainRunModel) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render("🌧  Rain in progress")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("↑/↓ select · s skip running repo · a/q abort all\n\n")

	for i, row := range m.rows {
		line := m.formatRow(row, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m rainRunModel) formatRow(row rainRepoRow, selected bool) string {
	label := row.name
	if label == "" {
		label = row.path
	}
	elapsed := ""
	overdue := false
	if row.state == rainexec.StateRunning || row.state == rainexec.StateRetrying {
		if !row.startedAt.IsZero() {
			sec := int(time.Since(row.startedAt).Seconds())
			elapsed = fmt.Sprintf(" %ds", sec)
			if m.timeoutSec > 0 && sec >= m.timeoutSec {
				overdue = true
			}
		}
	}
	status := string(row.state)
	switch row.state {
	case rainexec.StateRetrying:
		status = fmt.Sprintf("retrying %d/%d", row.attempt, row.maxAttempts)
	case rainexec.StateRunning:
		status = "running" + elapsed
	case rainexec.StateTimedOut:
		status = "timed out"
	case rainexec.StateFrozen:
		status = "frozen"
	case rainexec.StateFailed:
		status = "failed"
	case rainexec.StateDone:
		status = "done"
	case rainexec.StateSkipped:
		status = "skipped"
	case rainexec.StateQueued:
		status = "queued"
	}

	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	// Truncate plain text first; runewidth treats ANSI CSI bytes as width and
	// can clip a lipgloss reset if styling is applied before truncation.
	line := runewidth.Truncate(prefix+label+" — "+status, max(40, m.width-2), "…")
	if overdue {
		line = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(line)
	}
	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
