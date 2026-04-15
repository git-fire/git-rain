package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func revParse(t *testing.T, repoPath, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return string(out[:len(out)-1]) // trim newline
}

func TestMainlineFetchRemotes_UpdatesRemoteTrackingRefNotLocalBranch(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("mainline-fetch").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	localBefore := testutil.GetCurrentSHA(t, repo.Path())
	originBefore := revParse(t, repo.Path(), "origin/"+defaultBranch)

	cloneBase := t.TempDir()
	peerDir := filepath.Join(cloneBase, "peer")
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(peerDir, "remote-only.txt"), []byte("from remote\n"), 0o644); err != nil {
		t.Fatalf("write peer file: %v", err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "remote-only.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "remote advance")
	testutil.RunGitCmd(t, peerDir, "push", "origin", defaultBranch)

	originStale := revParse(t, repo.Path(), "origin/"+defaultBranch)
	if originStale != originBefore {
		t.Fatalf("expected origin/%s to match pre-push SHA before fetch", defaultBranch)
	}

	result, err := MainlineFetchRemotes(repo.Path(), RainOptions{})
	if err != nil {
		t.Fatalf("MainlineFetchRemotes() error = %v", err)
	}
	if result.Updated == 0 {
		t.Fatalf("expected at least one fetched branch, got %+v", result.Branches)
	}

	originAfter := revParse(t, repo.Path(), "origin/"+defaultBranch)
	if originAfter == originBefore {
		t.Fatalf("expected origin/%s to advance after fetch", defaultBranch)
	}
	localAfter := testutil.GetCurrentSHA(t, repo.Path())
	if localAfter != localBefore {
		t.Fatalf("mainline fetch should not move local branch (before=%s after=%s)", localBefore, localAfter)
	}
}
