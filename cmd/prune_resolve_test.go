package cmd

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"

	testutil "github.com/git-fire/git-testkit"
)

func TestApplyRepoFetchPrune_GitConfigOverridesGlobalOff(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-git-override").
		AddFile("a.txt", "x\n").
		Commit("init")
	abs, err := filepath.Abs(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	setCfg := exec.Command("git", "config", "--local", "--bool", git.RainFetchPruneConfigKey, "true")
	setCfg.Dir = repo.Path()
	if out, err := setCfg.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}

	resetFlags()
	reg := &registry.Registry{}
	base := git.RainOptions{FetchPrune: false}
	got, err := applyRepoFetchPrune(repo.Path(), base, abs, reg)
	if err != nil {
		t.Fatalf("applyRepoFetchPrune: %v", err)
	}
	if !got.FetchPrune {
		t.Fatal("repo git config rain.fetchprune=true should override global FetchPrune=false")
	}
}

func TestApplyRepoFetchPrune_RegistryOverridesGlobalOffWhenGitUnset(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-reg-override").
		AddFile("a.txt", "x\n").
		Commit("init")
	abs, err := filepath.Abs(repo.Path())
	if err != nil {
		t.Fatal(err)
	}

	resetFlags()
	v := true
	reg := &registry.Registry{
		Repos: []registry.RegistryEntry{
			{Path: abs, Name: "r", Status: registry.StatusActive, FetchPrune: &v},
		},
	}
	base := git.RainOptions{FetchPrune: false}
	got, err := applyRepoFetchPrune(repo.Path(), base, abs, reg)
	if err != nil {
		t.Fatalf("applyRepoFetchPrune: %v", err)
	}
	if !got.FetchPrune {
		t.Fatal("registry fetch_prune=true should override global FetchPrune=false when git unset")
	}
}

func TestApplyRepoFetchPrune_GitOverridesRegistry(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-git-beats-reg").
		AddFile("a.txt", "x\n").
		Commit("init")
	abs, err := filepath.Abs(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	setCfg := exec.Command("git", "config", "--local", "--bool", git.RainFetchPruneConfigKey, "false")
	setCfg.Dir = repo.Path()
	if out, err := setCfg.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}

	resetFlags()
	v := true
	reg := &registry.Registry{
		Repos: []registry.RegistryEntry{
			{Path: abs, Name: "r", Status: registry.StatusActive, FetchPrune: &v},
		},
	}
	base := git.RainOptions{FetchPrune: true}
	got, err := applyRepoFetchPrune(repo.Path(), base, abs, reg)
	if err != nil {
		t.Fatalf("applyRepoFetchPrune: %v", err)
	}
	if got.FetchPrune {
		t.Fatal("git rain.fetchprune=false should override registry true and global true")
	}
}

func TestApplyRepoFetchPrune_CLIOverridesGit(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-cli").
		AddFile("a.txt", "x\n").
		Commit("init")
	abs, err := filepath.Abs(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	setCfg := exec.Command("git", "config", "--local", "--bool", git.RainFetchPruneConfigKey, "false")
	setCfg.Dir = repo.Path()
	if out, err := setCfg.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}

	resetFlags()
	rainPrune = true
	reg := &registry.Registry{}
	base := git.RainOptions{FetchPrune: false}
	got, err := applyRepoFetchPrune(repo.Path(), base, abs, reg)
	if err != nil {
		t.Fatalf("applyRepoFetchPrune: %v", err)
	}
	if !got.FetchPrune {
		t.Fatal("--prune should override git rain.fetchprune=false")
	}
}

func TestApplyRepoFetchPrune_NoPruneOverridesGitTrue(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("prune-no-cli").
		AddFile("a.txt", "x\n").
		Commit("init")
	abs, err := filepath.Abs(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	setCfg := exec.Command("git", "config", "--local", "--bool", git.RainFetchPruneConfigKey, "true")
	setCfg.Dir = repo.Path()
	if out, err := setCfg.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}

	resetFlags()
	rainNoPrune = true
	reg := &registry.Registry{}
	base := git.RainOptions{FetchPrune: true}
	got, err := applyRepoFetchPrune(repo.Path(), base, abs, reg)
	if err != nil {
		t.Fatalf("applyRepoFetchPrune: %v", err)
	}
	if got.FetchPrune {
		t.Fatal("--no-prune should force off even when git and global want prune")
	}
}

func TestRunRain_PruneAndNoPruneTogetherErrors(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)
	resetFlags()
	rainPrune = true
	rainNoPrune = true
	if err := runRain(rootCmd, []string{}); err == nil {
		t.Fatal("expected error when --prune and --no-prune both set")
	}
}
