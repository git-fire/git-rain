package cmd

import (
	"context"
	"testing"

	"github.com/git-rain/git-rain/internal/rainexec"
)

func TestClassifyRepoRainOutcome_SkipDoesNotCountFrozen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state, delta := classifyRepoRainOutcome(ctx, rainTotals{frozen: 1}, jsonRepoResult{Error: "context canceled"})
	if state != rainexec.StateSkipped {
		t.Fatalf("state = %q, want skipped", state)
	}
	if delta.frozen != 0 {
		t.Fatalf("frozen = %d, want 0", delta.frozen)
	}
}

func TestClassifyRepoRainOutcome_SkipAfterWorkShowsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state, delta := classifyRepoRainOutcome(ctx, rainTotals{updated: 1, frozen: 1}, jsonRepoResult{})
	if state != rainexec.StateDone {
		t.Fatalf("state = %q, want done", state)
	}
	if delta.frozen != 1 {
		t.Fatalf("delta frozen unchanged when work completed")
	}
}

func TestClassifyRepoRainOutcome_TimedOutFromReportError(t *testing.T) {
	// Quiet TUI path never fills the human buffer; Error still carries the timeout.
	state, _ := classifyRepoRainOutcome(context.Background(), rainTotals{frozen: 1}, jsonRepoResult{
		Error: "timed out after 120s",
	})
	if state != rainexec.StateTimedOut {
		t.Fatalf("state = %q, want timed_out", state)
	}
}

func TestClassifyRepoRainOutcome_TimedOutFromBranchMessage(t *testing.T) {
	state, _ := classifyRepoRainOutcome(context.Background(), rainTotals{frozen: 1}, jsonRepoResult{
		Branches: []jsonBranchResult{{Message: "timed out after 30s"}},
	})
	if state != rainexec.StateTimedOut {
		t.Fatalf("state = %q, want timed_out", state)
	}
}

func TestClassifyRepoRainOutcome_FrozenWithoutTimeout(t *testing.T) {
	state, _ := classifyRepoRainOutcome(context.Background(), rainTotals{frozen: 1}, jsonRepoResult{
		Error: "could not reach remote — check your network and try again",
	})
	if state != rainexec.StateFrozen {
		t.Fatalf("state = %q, want frozen", state)
	}
}
