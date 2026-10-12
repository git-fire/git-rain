package cmd

import (
	"context"
	"errors"
	"strings"

	"github.com/git-rain/git-rain/internal/rainexec"
)

// classifyRepoRainOutcome maps runRainOnRepo totals to a TUI state and adjusts
// counters when a per-repo cancel should count as skipped, not frozen.
// Timeout detection uses the structured report (Error / branch Messages) so it
// works when human CLI output is suppressed (quiet TUI rain phase).
func classifyRepoRainOutcome(repoCtx context.Context, delta rainTotals, report jsonRepoResult) (rainexec.RepoState, rainTotals) {
	cancelled := repoCtx != nil && errors.Is(repoCtx.Err(), context.Canceled)
	workDone := delta.updated > 0 || delta.skipped > 0

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
		if repoRainTimedOut(report) {
			return rainexec.StateTimedOut, delta
		}
		return rainexec.StateFrozen, delta
	default:
		return rainexec.StateDone, delta
	}
}

func repoRainTimedOut(report jsonRepoResult) bool {
	if strings.Contains(report.Error, "timed out") {
		return true
	}
	for _, br := range report.Branches {
		if strings.Contains(br.Message, "timed out") {
			return true
		}
	}
	return false
}
