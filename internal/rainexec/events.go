package rainexec

import "time"

// RepoState is the rain-phase status for one repository.
type RepoState string

const (
	StateQueued    RepoState = "queued"
	StateRunning   RepoState = "running"
	StateRetrying  RepoState = "retrying"
	StateTimedOut  RepoState = "timed_out"
	StateFrozen    RepoState = "frozen"
	StateFailed    RepoState = "failed"
	StateDone      RepoState = "done"
	StateSkipped   RepoState = "skipped"
)

// Event is a rain-phase progress update for the TUI or tests.
type Event struct {
	RepoPath    string
	RepoName    string
	State       RepoState
	Attempt     int
	MaxAttempts int
	StartedAt   time.Time
	Detail      string
}
