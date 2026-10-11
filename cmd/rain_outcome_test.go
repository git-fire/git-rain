package cmd

import (
	"context"
	"testing"

	"github.com/git-rain/git-rain/internal/rainexec"
)

func TestClassifyRepoRainOutcome_SkipDoesNotCountFrozen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state, delta := classifyRepoRainOutcome(ctx, rainTotals{frozen: 1}, "context canceled")
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
	state, delta := classifyRepoRainOutcome(ctx, rainTotals{updated: 1, frozen: 1}, "fetched")
	if state != rainexec.StateDone {
		t.Fatalf("state = %q, want done", state)
	}
	if delta.frozen != 1 {
		t.Fatalf("delta frozen unchanged when work completed")
	}
}
