package git

import (
	"os/exec"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func TestReadRainFetchPrune_Unset(t *testing.T) {
	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-unset").AddFile("a.txt", "x\n").Commit("init")
	set, val, err := ReadRainFetchPrune(repo.Path())
	if err != nil {
		t.Fatalf("ReadRainFetchPrune: %v", err)
	}
	if set {
		t.Fatal("expected unset")
	}
	if val {
		t.Fatal("val should be false when unset")
	}
}

func TestReadRainFetchPrune_SetTrueAndFalse(t *testing.T) {
	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-set").AddFile("a.txt", "x\n").Commit("init")
	path := repo.Path()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("config", "--local", "--bool", RainFetchPruneConfigKey, "true")
	set, val, err := ReadRainFetchPrune(path)
	if err != nil || !set || !val {
		t.Fatalf("want set true, got set=%v val=%v err=%v", set, val, err)
	}
	run("config", "--local", "--bool", RainFetchPruneConfigKey, "false")
	set, val, err = ReadRainFetchPrune(path)
	if err != nil || !set || val {
		t.Fatalf("want set false, got set=%v val=%v err=%v", set, val, err)
	}
}

func TestReadRainFetchPrune_RepoPathUsesLocalConfig(t *testing.T) {
	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-local").AddFile("a.txt", "x\n").Commit("init")
	otherDir := t.TempDir()
	cmd := exec.Command("git", "config", "--local", "--bool", RainFetchPruneConfigKey, "true")
	cmd.Dir = repo.Path()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}
	// Wrong directory should not see the repo's local config
	set, _, err := ReadRainFetchPrune(otherDir)
	if err != nil {
		t.Fatalf("ReadRainFetchPrune: %v", err)
	}
	if set {
		t.Fatal("other dir should not have rain.fetchprune set")
	}
}
