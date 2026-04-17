package git

import (
	"context"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

// TestScanRepositoriesStream_PreCancelledDrain verifies that when the scan
// context is already cancelled, ScanRepositoriesStream still closes the output
// channel so callers never block on range. This guards the --rain quit path
// where cancelScan runs while the picker exits.
func TestScanRepositoriesStream_PreCancelledDrain(t *testing.T) {
	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("scan-pre-cancel").
		AddFile("README.md", "x\n").
		Commit("init")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := make(chan Repository, 8)
	opts := ScanOptions{
		RootPath:       repo.Path(),
		Workers:        2,
		Ctx:            ctx,
		MaxDepth:       4,
		DisableScan:    true,
		KnownPaths:     map[string]bool{repo.Path(): false},
		FolderProgress: nil,
	}

	err := ScanRepositoriesStream(opts, out)
	if err != nil {
		t.Fatalf("ScanRepositoriesStream: %v", err)
	}

	for range out {
	}
}
