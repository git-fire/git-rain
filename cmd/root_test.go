package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func resetFlags() {
	rainPath = ""
	rainNoScan = false
	rainRisky = false
	rainDryRun = false
	rainMainlineFetch = false
	rainInit = false
	rainConfigFile = ""
	rainRain = false
	rainSync = false
	forceUnlockRegistry = false
}

func setTestUserDirs(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureStdout: pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("captureStdout: read: %v", err)
	}
	return buf.String()
}

func TestRootCommand_FlagParsing_Risky(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{"--risky"}); err != nil {
		t.Fatalf("rootCmd.ParseFlags(--risky) error = %v", err)
	}
	if !rainRisky {
		t.Fatal("expected rainRisky flag to be set")
	}
}

func TestRootCommand_FlagParsing_MainlineFetch(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{"--mainline-fetch"}); err != nil {
		t.Fatalf("rootCmd.ParseFlags(--mainline-fetch) error = %v", err)
	}
	if !rainMainlineFetch {
		t.Fatal("expected rainMainlineFetch flag to be set")
	}
}

func TestComputeFullSync(t *testing.T) {
	tests := []struct {
		name                  string
		explicitSync          bool
		riskyFlag, riskyCfg   bool
		branchFlag, branchCfg string
		want                  bool
	}{
		{"explicit sync", true, false, false, "", "", true},
		{"risky flag", false, true, false, "", "", true},
		{"risky cfg", false, false, true, "", "", true},
		{"branch flag set", false, false, false, "checked-out", "", true},
		{"mainline config only", false, false, false, "", "mainline", false},
		{"empty branch config", false, false, false, "", "", false},
		{"all-local config", false, false, false, "", "all-local", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeFullSync(tt.explicitSync, tt.riskyFlag, tt.riskyCfg, tt.branchFlag, tt.branchCfg)
			if got != tt.want {
				t.Fatalf("computeFullSync(...) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunRain_DefaultFetchAllDoesNotMoveLocalBranch(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-default-fetch").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)
	localSHA := testutil.GetCurrentSHA(t, repo.Path())

	resetFlags()
	rainPath = filepath.Dir(repo.Path())

	var runErr error
	output := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain() default fetch error = %v", runErr)
	}
	if !strings.Contains(output, "git fetch --all --prune per repo") {
		t.Fatalf("expected default fetch-all banner, got:\n%s", output)
	}
	if !strings.Contains(output, "fetched") {
		t.Fatalf("expected fetched line, got:\n%s", output)
	}
	if got := testutil.GetCurrentSHA(t, repo.Path()); got != localSHA {
		t.Fatalf("default fetch-all should not move local branch (want=%s got=%s)", localSHA, got)
	}
}

func TestRunRain_SafeModeSkipsLocalAheadBranch(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-safe-cmd").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)
	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")
	localAheadSHA := testutil.GetCurrentSHA(t, repo.Path())

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainSync = true

	var runErr error
	output := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain() safe mode error = %v", runErr)
	}
	if !strings.Contains(output, "local ahead") {
		t.Fatalf("expected safe mode output to mention 'local ahead', got:\n%s", output)
	}
	if got := testutil.GetCurrentSHA(t, repo.Path()); got != localAheadSHA {
		t.Fatalf("safe mode should preserve local-ahead SHA (want=%s got=%s)", localAheadSHA, got)
	}
}

func TestRunRain_RiskyFlagResetsLocalAheadBranch(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote")
	repo := scenario.CreateRepo("rain-risky-cmd").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	defaultBranch := repo.GetDefaultBranch()
	repo.Push("origin", defaultBranch)
	remoteSHA := testutil.GetCurrentSHA(t, repo.Path())

	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")
	if aheadSHA := testutil.GetCurrentSHA(t, repo.Path()); aheadSHA == remoteSHA {
		t.Fatal("test setup error: local-ahead SHA must differ from remote SHA")
	}

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainRisky = true
	rainSync = true

	var runErr error
	output := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain() risky mode error = %v", runErr)
	}
	if !strings.Contains(output, "realigned") {
		t.Fatalf("expected risky output to mention 'realigned', got:\n%s", output)
	}
	if got := testutil.GetCurrentSHA(t, repo.Path()); got != remoteSHA {
		t.Fatalf("risky mode should reset branch to remote SHA (want=%s got=%s)", remoteSHA, got)
	}
	if !hasRainBackupBranch(t, repo.Path()) {
		t.Fatal("expected runRain risky mode to create a backup branch")
	}
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
