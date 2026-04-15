package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func TestRainRepository_FastForwardsCurrentBranchPreservingUnstaged(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-current").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

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

	if err := os.WriteFile(filepath.Join(repo.Path(), "wip.txt"), []byte("local wip\n"), 0o644); err != nil {
		t.Fatalf("write local wip: %v", err)
	}

	result, err := RainRepository(repo.Path(), RainOptions{})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}
	if result.Updated == 0 {
		t.Fatalf("expected at least one updated branch, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(repo.Path(), "remote-only.txt")); statErr != nil {
		t.Fatal("expected remote commit to be fast-forwarded into current branch")
	}
	if _, statErr := os.Stat(filepath.Join(repo.Path(), "wip.txt")); statErr != nil {
		t.Fatal("expected unstaged local file to be preserved")
	}
}

func TestRainRepository_SafeModeSkipsLocalAhead(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-safe-ahead").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)
	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")
	localSHABefore := testutil.GetCurrentSHA(t, repo.Path())

	result, err := RainRepository(repo.Path(), RainOptions{RiskyMode: false})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}
	localSHANow := testutil.GetCurrentSHA(t, repo.Path())
	if localSHANow != localSHABefore {
		t.Fatalf("safe mode should not rewrite local-ahead branch (before=%s after=%s)", localSHABefore, localSHANow)
	}
	if !containsRainOutcome(result, RainOutcomeSkippedLocalAhead) {
		t.Fatalf("expected skipped-local-ahead outcome, got %+v", result.Branches)
	}
}

func TestRainRepository_RiskyModeResetsCurrentBranchWhenClean(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-risky-reset").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)
	remoteSHA := testutil.GetCurrentSHA(t, repo.Path())

	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")
	localAheadSHA := testutil.GetCurrentSHA(t, repo.Path())
	if localAheadSHA == remoteSHA {
		t.Fatal("test setup error: local ahead SHA should differ from remote")
	}

	result, err := RainRepository(repo.Path(), RainOptions{RiskyMode: true})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}
	currentSHA := testutil.GetCurrentSHA(t, repo.Path())
	if currentSHA != remoteSHA {
		t.Fatalf("expected risky mode reset current branch to upstream SHA=%s, got %s", remoteSHA, currentSHA)
	}
	if !containsRainOutcome(result, RainOutcomeUpdatedRisky) {
		t.Fatalf("expected updated-risky outcome, got %+v", result.Branches)
	}
	if !hasRainBackupBranch(t, repo.Path()) {
		t.Fatal("expected risky mode to create a git-rain-backup-* branch")
	}
}

func TestRainRepository_RiskyModeSkipsDirtyCurrentBranch(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-risky-dirty").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")
	localAheadSHA := testutil.GetCurrentSHA(t, repo.Path())

	if err := os.WriteFile(filepath.Join(repo.Path(), "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("write staged: %v", err)
	}
	testutil.RunGitCmd(t, repo.Path(), "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(repo.Path(), "unstaged.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatalf("write unstaged: %v", err)
	}

	result, err := RainRepository(repo.Path(), RainOptions{RiskyMode: true})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}
	currentSHA := testutil.GetCurrentSHA(t, repo.Path())
	if currentSHA != localAheadSHA {
		t.Fatalf("dirty current branch should not be rewritten (before=%s after=%s)", localAheadSHA, currentSHA)
	}
	if !containsRainOutcome(result, RainOutcomeSkippedUnsafeDirty) {
		t.Fatalf("expected skipped-unsafe-dirty outcome, got %+v", result.Branches)
	}
}

func TestRainRepository_UpdatesNonCurrentBranchWhileCurrentDirty(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-noncurrent").
		WithRemote("origin", remote).
		AddFile("main.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	testutil.RunGitCmd(t, repo.Path(), "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo.Path(), "feature.txt"), []byte("feature v1\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	testutil.RunGitCmd(t, repo.Path(), "add", "feature.txt")
	testutil.RunGitCmd(t, repo.Path(), "commit", "-m", "feature init")
	repo.Push("origin", "feature")
	featureBefore := testutil.GetCurrentSHA(t, repo.Path())

	cloneBase := t.TempDir()
	peerDir := filepath.Join(cloneBase, "peer")
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test User")
	testutil.RunGitCmd(t, peerDir, "checkout", "feature")
	if err := os.WriteFile(filepath.Join(peerDir, "feature.txt"), []byte("feature v2\n"), 0o644); err != nil {
		t.Fatalf("write peer feature update: %v", err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "feature.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "feature remote advance")
	testutil.RunGitCmd(t, peerDir, "push", "origin", "feature")

	testutil.RunGitCmd(t, repo.Path(), "checkout", defaultBranch)
	if err := os.WriteFile(filepath.Join(repo.Path(), "wip.txt"), []byte("do not lose me\n"), 0o644); err != nil {
		t.Fatalf("write local wip: %v", err)
	}

	result, err := RainRepository(repo.Path(), RainOptions{BranchMode: BranchSyncAllLocal})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}

	testutil.RunGitCmd(t, repo.Path(), "checkout", "feature")
	featureAfter := testutil.GetCurrentSHA(t, repo.Path())
	if featureAfter == featureBefore {
		t.Fatalf("expected feature branch to update from remote while current branch is dirty")
	}
	testutil.RunGitCmd(t, repo.Path(), "checkout", defaultBranch)
	if _, statErr := os.Stat(filepath.Join(repo.Path(), "wip.txt")); statErr != nil {
		t.Fatal("expected dirty current-branch worktree file to remain after rain")
	}
	if !containsRainOutcome(result, RainOutcomeUpdated) {
		t.Fatalf("expected at least one updated outcome, got %+v", result.Branches)
	}
}

func TestRainRepository_MainlineMode_IgnoresNonMainlineBranches(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-mainline").
		WithRemote("origin", remote).
		AddFile("main.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	// Create a feature branch locally and push it.
	testutil.RunGitCmd(t, repo.Path(), "checkout", "-b", "feature/cool-thing")
	repo.AddFile("feat.txt", "feat\n").Commit("feature commit")
	repo.Push("origin", "feature/cool-thing")

	// Advance the remote default branch via a peer.
	cloneBase := t.TempDir()
	peerDir := cloneBase + "/peer"
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test")
	if err := os.WriteFile(peerDir+"/remote.txt", []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "remote.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "remote advance")
	testutil.RunGitCmd(t, peerDir, "push", "origin", defaultBranch)

	// Switch back to default branch so feature is non-current.
	testutil.RunGitCmd(t, repo.Path(), "checkout", defaultBranch)

	result, err := RainRepository(repo.Path(), RainOptions{BranchMode: BranchSyncMainline})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}

	// Default branch should be updated.
	if !containsRainOutcome(result, RainOutcomeUpdated) {
		t.Fatalf("expected mainline branch to be updated, got %+v", result.Branches)
	}

	// feature/cool-thing should not appear in results at all.
	for _, br := range result.Branches {
		if br.Branch == "feature/cool-thing" {
			t.Fatalf("mainline mode should not process feature branches, got %+v", br)
		}
	}
}

func TestRainRepository_MainlineMode_UserPatternsExtendMainline(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-patterns").
		WithRemote("origin", remote).
		AddFile("main.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	// Create a JIRA-prefixed branch.
	testutil.RunGitCmd(t, repo.Path(), "checkout", "-b", "JIRA-123")
	repo.AddFile("jira.txt", "work\n").Commit("jira work")
	repo.Push("origin", "JIRA-123")

	// Advance JIRA branch from peer.
	cloneBase := t.TempDir()
	peerDir := cloneBase + "/peer"
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test")
	testutil.RunGitCmd(t, peerDir, "checkout", "JIRA-123")
	if err := os.WriteFile(peerDir+"/jira2.txt", []byte("more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "jira2.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "jira advance")
	testutil.RunGitCmd(t, peerDir, "push", "origin", "JIRA-123")

	testutil.RunGitCmd(t, repo.Path(), "checkout", "JIRA-123")
	jiraBefore := testutil.GetCurrentSHA(t, repo.Path())
	testutil.RunGitCmd(t, repo.Path(), "checkout", defaultBranch)

	result, err := RainRepository(repo.Path(), RainOptions{
		BranchMode:       BranchSyncMainline,
		MainlinePatterns: []string{"JIRA-"},
	})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}

	testutil.RunGitCmd(t, repo.Path(), "checkout", "JIRA-123")
	jiraAfter := testutil.GetCurrentSHA(t, repo.Path())
	if jiraAfter == jiraBefore {
		t.Fatalf("JIRA-123 branch should have been synced via mainline_patterns (before=%s after=%s)", jiraBefore, jiraAfter)
	}
	_ = result
}

func TestRainRepository_CheckedOutMode_OnlySyncsCheckedOutBranch(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-checkedout").
		WithRemote("origin", remote).
		AddFile("main.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	// Create a secondary branch locally and push it.
	testutil.RunGitCmd(t, repo.Path(), "checkout", "-b", "secondary")
	repo.AddFile("sec.txt", "v1\n").Commit("secondary init")
	repo.Push("origin", "secondary")

	// Advance secondary from a peer.
	cloneBase := t.TempDir()
	peerDir := cloneBase + "/peer"
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test")
	testutil.RunGitCmd(t, peerDir, "checkout", "secondary")
	if err := os.WriteFile(peerDir+"/sec2.txt", []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "sec2.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "secondary advance")
	testutil.RunGitCmd(t, peerDir, "push", "origin", "secondary")

	// Stay on default branch; secondary is not checked out.
	testutil.RunGitCmd(t, repo.Path(), "checkout", defaultBranch)

	result, err := RainRepository(repo.Path(), RainOptions{BranchMode: BranchSyncCheckedOut})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}

	// secondary was not checked out so should not appear in results.
	for _, br := range result.Branches {
		if br.Branch == "secondary" {
			t.Fatalf("checked-out mode should not process non-checked-out branch secondary, got %+v", br)
		}
	}
}

func TestRainRepository_AllBranchesMode_CreatesLocalTrackingRefs(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-allbranches").
		WithRemote("origin", remote).
		AddFile("main.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)

	// Push a branch to the remote that the local repo does not know about.
	cloneBase := t.TempDir()
	peerDir := cloneBase + "/peer"
	testutil.RunGitCmd(t, cloneBase, "clone", remote.Path(), peerDir)
	testutil.RunGitCmd(t, peerDir, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, peerDir, "config", "user.name", "Test")
	testutil.RunGitCmd(t, peerDir, "checkout", "-b", "remote-only")
	if err := os.WriteFile(peerDir+"/remote-only.txt", []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGitCmd(t, peerDir, "add", "remote-only.txt")
	testutil.RunGitCmd(t, peerDir, "commit", "-m", "remote-only branch")
	testutil.RunGitCmd(t, peerDir, "push", "origin", "remote-only")

	// Confirm local does not have the branch yet.
	cmd := exec.Command("git", "branch")
	cmd.Dir = repo.Path()
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "remote-only") {
		t.Fatal("test setup: local should not have remote-only branch before rain")
	}

	_, err := RainRepository(repo.Path(), RainOptions{BranchMode: BranchSyncAllBranches})
	if err != nil {
		t.Fatalf("RainRepository() error = %v", err)
	}

	// Confirm local now has the branch.
	cmd = exec.Command("git", "branch")
	cmd.Dir = repo.Path()
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "remote-only") {
		t.Fatal("all-branches mode should have created local tracking branch for remote-only")
	}
}

func containsRainOutcome(result RainResult, outcome string) bool {
	for _, br := range result.Branches {
		if br.Outcome == outcome {
			return true
		}
	}
	return false
}

func hasRainBackupBranch(t *testing.T, repoPath string) bool {
	t.Helper()
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch listing failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "git-rain-backup-") {
			return true
		}
	}
	return false
}
