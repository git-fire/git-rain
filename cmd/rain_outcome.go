package cmd

import (
	"context"
	"errors"
	"strings"

	"github.com/git-rain/git-rain/internal/rainexec"
)

// classifyRepoRainOutcome maps runRainOnRepo totals to a TUI state and adjusts
// counters when a per-repo cancel should count as skipped, not frozen.
func classifyRepoRainOutcome(repoCtx context.Context, delta rainTotals, output string) (rainexec.RepoState, rainTotals) {
	cancelled := repoCtx != nil && errors.Is(repoCtx.Err(), context.Canceled)
	// Only successful updates count as completed work. Branch skips (no-upstream,
	// etc.) are recorded before fetch and must not mask a user cancel as done.
	workDone := delta.updated > 0

	if cancelled {
		if workDone {
			return rainexec.StateDone, delta
		}
		d := delta
		d.frozen = 0
		d.failed = 0
		return rainexec.StateSkipped, d
	}

	switch {
	case delta.failed > 0:
		return rainexec.StateFailed, delta
	case delta.frozen > 0:
		if strings.Contains(output, "timed out") {
			return rainexec.StateTimedOut, delta
		}
		return rainexec.StateFrozen, delta
	default:
		return rainexec.StateDone, delta
	}
}
