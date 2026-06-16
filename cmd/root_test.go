package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testutil "github.com/git-fire/git-testkit"

	"github.com/git-rain/git-rain/internal/config"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
)

func resetFlags() {
	rainPath = ""
	rainNoScan = false
	rainRisky = false
	rainDryRun = false
	rainFetchMainline = false
	rainInit = false
	rainConfigFile = ""
	rainRain = false
	rainSync = false
	rainPrune = false
	rainNoPrune = false
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

func TestRootCommand_FlagParsing_FetchMainline(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{"--fetch-mainline"}); err != nil {
		t.Fatalf("rootCmd.ParseFlags(--fetch-mainline) error = %v", err)
	}
	if !rainFetchMainline {
		t.Fatal("expected rainFetchMainline flag to be set")
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

func TestRunRain_DryRunSyncShowsPruneResolution(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := scenario.CreateRepo("dry-sync-prune").
		AddFile("a.txt", "x\n").
		Commit("init")

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainDryRun = true
	rainSync = true
	rainPrune = true

	var runErr error
	output := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain() dry-run sync error = %v", runErr)
	}
	if !strings.Contains(output, "Fetch --prune: on for this run (--prune)") {
		t.Fatalf("expected dry-run to show prune on for --sync path, got:\n%s", output)
	}
	if !strings.Contains(output, "Would hydrate") {
		t.Fatalf("expected dry-run hydrate wording for --sync, got:\n%s", output)
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
	if !strings.Contains(output, "git fetch --all per repo") {
		t.Fatalf("expected default full-fetch banner, got:\n%s", output)
	}
	if !strings.Contains(output, "fetched") {
		t.Fatalf("expected fetched line, got:\n%s", output)
	}
	if got := testutil.GetCurrentSHA(t, repo.Path()); got != localSHA {
		t.Fatalf("default full fetch should not move local branch (want=%s got=%s)", localSHA, got)
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

// makeFetchedRepo creates a git repo with a remote that has been pushed to,
// returning the repo for use in tests that need the default fetch path.
func makeFetchedRepo(t *testing.T, scenario *testutil.Scenario, name string) *testutil.ScenarioRepo {
	t.Helper()
	remote := scenario.CreateBareRepo(name + "-remote")
	repo := scenario.CreateRepo(name).
		WithRemote("origin", remote).
		AddFile("a.txt", "v1\n").
		Commit("init")
	repo.Push("origin", repo.GetDefaultBranch())
	return repo
}

// cloneIntoScanRoot clones a bare remote into scanRoot/<name>. Returns the
// clone path. Used to put multiple repos under one scan root without
// relocating testkit-managed paths (which live under per-call t.TempDir()s).
func cloneIntoScanRoot(t *testing.T, scanRoot, name, remotePath string) string {
	t.Helper()
	dst := filepath.Join(scanRoot, name)
	out, err := exec.Command("git", "clone", remotePath, dst).CombinedOutput()
	if err != nil {
		t.Fatalf("git clone %s into %s failed: %v\n%s", remotePath, dst, err, out)
	}
	testutil.RunGitCmd(t, dst, "config", "user.email", "test@example.com")
	testutil.RunGitCmd(t, dst, "config", "user.name", "Test User")
	return dst
}

func TestRunRain_DefaultStream_MultiRepoParity(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)
	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "1")

	scenario := testutil.NewScenario(t)
	scanRoot := t.TempDir()

	for i := 0; i < 3; i++ {
		seed := scenario.CreateRepo("seed-"+string(rune('a'+i))).
			AddFile("a.txt", "v1\n").
			Commit("init")
		remote := scenario.CreateBareRepo("remote-" + string(rune('a'+i)))
		seed.WithRemote("origin", remote).Push("origin", seed.GetDefaultBranch())
		cloneIntoScanRoot(t, scanRoot, "stream-multi-"+string(rune('a'+i)), remote.Path())
	}

	resetFlags()
	rainPath = scanRoot

	var runErr error
	out := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain default stream error = %v\n%s", runErr, out)
	}

	// All three repos should have a "fetched" line.
	if got := strings.Count(out, "↓  fetched"); got != 3 {
		t.Fatalf("want exactly 3 'fetched' lines, got %d. output:\n%s", got, out)
	}

	// Each repo's [N/M] header should be on its own line and immediately
	// followed by its fetched line — that confirms the printMu serialization
	// kept the per-repo block contiguous.
	lines := strings.Split(out, "\n")
	headerCount := 0
	for i, line := range lines {
		if !strings.Contains(line, "] stream-multi-") {
			continue
		}
		headerCount++
		if i+1 >= len(lines) || !strings.Contains(lines[i+1], "↓  fetched") {
			t.Fatalf("expected fetched line directly after header %q; got %q. full output:\n%s",
				line, safeIndex(lines, i+1), out)
		}
	}
	if headerCount != 3 {
		t.Fatalf("want 3 [N/M] headers, got %d. output:\n%s", headerCount, out)
	}

	if !strings.Contains(out, "rain delivered") {
		t.Fatalf("expected summary line, got:\n%s", out)
	}
}

func safeIndex(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return "<out of range>"
	}
	return s[i]
}

func TestRunRain_DefaultStream_NoScanHydratesRegistryOnly(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)
	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "1")

	scenario := testutil.NewScenario(t)
	repos := []*testutil.ScenarioRepo{
		makeFetchedRepo(t, scenario, "noscan-a"),
		makeFetchedRepo(t, scenario, "noscan-b"),
	}

	regPath, err := registry.DefaultRegistryPath()
	if err != nil {
		t.Fatalf("DefaultRegistryPath: %v", err)
	}
	reg := &registry.Registry{}
	now := time.Now()
	for _, r := range repos {
		abs, absErr := filepath.Abs(r.Path())
		if absErr != nil {
			t.Fatalf("abs: %v", absErr)
		}
		reg.Upsert(registry.RegistryEntry{
			Path:     abs,
			Name:     filepath.Base(abs),
			Status:   registry.StatusActive,
			Mode:     git.ModeSyncDefault.String(),
			AddedAt:  now,
			LastSeen: now,
		})
	}
	if err := registry.Save(reg, regPath); err != nil {
		t.Fatalf("registry.Save: %v", err)
	}

	resetFlags()
	rainNoScan = true
	rainPath = t.TempDir() // empty dir; no-scan should ignore it anyway

	var runErr error
	out := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain --no-scan error = %v\n%s", runErr, out)
	}

	if !strings.Contains(out, "Rain scanning disabled") {
		t.Fatalf("expected scanning-disabled banner, got:\n%s", out)
	}
	if got := strings.Count(out, "↓  fetched"); got != 2 {
		t.Fatalf("want 2 fetched lines for the 2 known repos, got %d. output:\n%s", got, out)
	}
	if strings.Contains(out, "🔍 Scanning…") {
		t.Fatalf("scan progress line should be absent under --no-scan. output:\n%s", out)
	}
}

func TestRunRain_DefaultStream_EmptyScanRoot(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)
	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "1")

	resetFlags()
	rainPath = t.TempDir()

	var runErr error
	out := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain empty-scan error = %v\n%s", runErr, out)
	}
	if !strings.Contains(out, "No git repositories found.") {
		t.Fatalf("expected 'No git repositories found.' in output, got:\n%s", out)
	}
	if strings.Contains(out, "rain delivered") {
		t.Fatalf("should not print summary when no repos. output:\n%s", out)
	}
}

func TestFetchWorkerCount(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to default", 0, config.DefaultFetchWorkers},
		{"negative falls back to default", -3, config.DefaultFetchWorkers},
		{"positive is preserved", 7, 7},
		{"one is preserved", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fetchWorkerCount(tt.in); got != tt.want {
				t.Fatalf("fetchWorkerCount(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestRunRain_DefaultStream_ZeroFetchWorkersStillRuns(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)
	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "1")

	scenario := testutil.NewScenario(t)
	repo := makeFetchedRepo(t, scenario, "zero-workers")

	cfgDir := filepath.Join(tmpHome, ".config", "git-rain")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	cfgBody := `[global]
scan_path = "."
fetch_workers = 0
`
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resetFlags()
	rainConfigFile = cfgPath
	rainPath = filepath.Dir(repo.Path())

	var runErr error
	out := captureStdout(t, func() {
		runErr = runRain(rootCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runRain zero-workers error = %v\n%s", runErr, out)
	}
	if !strings.Contains(out, "↓  fetched") {
		t.Fatalf("expected fetched line even when fetch_workers=0; got:\n%s", out)
	}
}

func TestFetchFailureMessage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"authentication", "Authentication failed for git@github.com", "could not authenticate with remote — check your credentials and try again"},
		{"permission denied", "git@github.com: Permission denied (publickey)", "could not authenticate with remote — check your credentials and try again"},
		{"could not read", "fatal: could not read from remote", "could not authenticate with remote — check your credentials and try again"},
		{"401", "fatal: HTTP 401: Unauthorized", "could not authenticate with remote — check your credentials and try again"},
		{"403", "fatal: HTTP 403 forbidden", "could not authenticate with remote — check your credentials and try again"},
		{"could not resolve", "fatal: unable to access ...: Could not resolve host", "could not reach remote — check your network and try again"},
		{"connection", "fatal: unable to access: connection refused", "could not reach remote — check your network and try again"},
		{"timed out", "fatal: connection timed out", "could not reach remote — check your network and try again"},
		{"network", "transient network failure", "could not reach remote — check your network and try again"},
		{"generic fallback", "something else went wrong", "fetch did not complete — try again when the remote is reachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fetchFailureMessage(tt.in); got != tt.want {
				t.Fatalf("fetchFailureMessage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWeatherSymbol(t *testing.T) {
	tests := []struct {
		outcome string
		want    string
	}{
		{git.RainOutcomeUpdated, "↓"},
		{git.RainOutcomeUpdatedRisky, "⚡"},
		{git.RainOutcomeUpToDate, "·"},
		{git.RainOutcomeFetched, "↓"},
		{git.RainOutcomeFrozen, "❄"},
		{git.RainOutcomeFailed, "✗"},
		{git.RainOutcomeSkippedNoUpstream, "~"},
		{git.RainOutcomeSkippedAmbiguousUpstream, "~"},
		{git.RainOutcomeSkippedUpstreamMissing, "~"},
		{git.RainOutcomeSkippedCheckedOut, "~"},
		{git.RainOutcomeSkippedLocalAhead, "~"},
		{git.RainOutcomeSkippedDiverged, "~"},
		{git.RainOutcomeSkippedUnsafeMerge, "~"},
		{git.RainOutcomeSkippedUnsafeDirty, "~"},
		{"unknown-outcome", "~"},
	}
	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			if got := weatherSymbol(tt.outcome); got != tt.want {
				t.Fatalf("weatherSymbol(%q) = %q, want %q", tt.outcome, got, tt.want)
			}
		})
	}
}

func TestOutcomeLabel(t *testing.T) {
	tests := []struct {
		outcome string
		want    string
	}{
		{git.RainOutcomeUpdated, "synced"},
		{git.RainOutcomeUpdatedRisky, "realigned"},
		{git.RainOutcomeUpToDate, "current"},
		{git.RainOutcomeFetched, "fetched"},
		{git.RainOutcomeFrozen, "frozen"},
		{git.RainOutcomeFailed, "failed"},
		{git.RainOutcomeSkippedNoUpstream, "no upstream"},
		{git.RainOutcomeSkippedAmbiguousUpstream, "ambiguous upstream"},
		{git.RainOutcomeSkippedUpstreamMissing, "upstream missing"},
		{git.RainOutcomeSkippedCheckedOut, "checked out elsewhere"},
		{git.RainOutcomeSkippedLocalAhead, "local ahead"},
		{git.RainOutcomeSkippedDiverged, "diverged"},
		{git.RainOutcomeSkippedUnsafeMerge, "unsafe merge"},
		{git.RainOutcomeSkippedUnsafeDirty, "dirty worktree"},
		{"made-up", "made-up"},
	}
	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			if got := outcomeLabel(tt.outcome); got != tt.want {
				t.Fatalf("outcomeLabel(%q) = %q, want %q", tt.outcome, got, tt.want)
			}
		})
	}
}

func TestBuildKnownPaths(t *testing.T) {
	rescanTrue := true
	rescanFalse := false

	tmp := t.TempDir()
	abs1 := filepath.Join(tmp, "abs1")
	abs2 := filepath.Join(tmp, "abs2")
	abs3 := filepath.Join(tmp, "abs3")
	abs4 := filepath.Join(tmp, "abs4")
	abs5 := filepath.Join(tmp, "abs5")

	reg := &registry.Registry{
		Repos: []registry.RegistryEntry{
			{Path: abs1, Status: registry.StatusActive},
			{Path: abs2, Status: registry.StatusMissing},
			{Path: abs3, Status: registry.StatusIgnored},
			{Path: abs4, Status: ""},
			{Path: abs5, Status: registry.StatusActive, RescanSubmodules: &rescanFalse},
			{Path: filepath.Join(tmp, "abs6"), Status: registry.StatusActive, RescanSubmodules: &rescanTrue},
		},
	}

	got := buildKnownPaths(reg, false)

	for _, p := range []string{abs1, abs2, abs4, abs5} {
		if _, ok := got[p]; !ok {
			t.Errorf("expected %s in known paths", p)
		}
	}
	if _, ok := got[abs3]; ok {
		t.Errorf("ignored entry %s should not be in known paths", abs3)
	}
	if got[abs5] != false {
		t.Errorf("entry-level rescan=false should override global=false (still false), got %v", got[abs5])
	}
	if got[filepath.Join(tmp, "abs6")] != true {
		t.Errorf("entry-level rescan=true should override global=false, got %v", got[filepath.Join(tmp, "abs6")])
	}

	// Now flip the global; the per-entry override on abs5 should still pin to false.
	gotGlobalOn := buildKnownPaths(reg, true)
	if gotGlobalOn[abs1] != true {
		t.Errorf("global=true should propagate to entries without override, got abs1=%v", gotGlobalOn[abs1])
	}
	if gotGlobalOn[abs5] != false {
		t.Errorf("global=true must NOT override per-entry rescan=false, got abs5=%v", gotGlobalOn[abs5])
	}
}

func TestUpsertRepoIntoRegistry(t *testing.T) {
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	tmp := t.TempDir()
	pathA := filepath.Join(tmp, "a")
	pathB := filepath.Join(tmp, "b")
	pathIgnored := filepath.Join(tmp, "ignored")

	reg := &registry.Registry{
		Repos: []registry.RegistryEntry{
			{Path: pathB, Name: "b", Status: registry.StatusActive, Mode: git.ModeSyncAll.String(), AddedAt: now.Add(-time.Hour), LastSeen: now.Add(-time.Hour)},
			{Path: pathIgnored, Name: "ignored", Status: registry.StatusIgnored, AddedAt: now.Add(-time.Hour)},
		},
	}

	// New entry should get default mode and IsNewRegistryEntry=true.
	gotA, includeA := upsertRepoIntoRegistry(reg, git.Repository{Path: pathA, Name: "a"}, now, git.ModeSyncDefault)
	if !includeA {
		t.Error("new entry should be included")
	}
	if !gotA.IsNewRegistryEntry {
		t.Error("new entry should report IsNewRegistryEntry=true")
	}
	if gotA.Mode != git.ModeSyncDefault {
		t.Errorf("new entry mode = %v, want %v", gotA.Mode, git.ModeSyncDefault)
	}
	if reg.FindByPath(pathA) == nil {
		t.Error("new entry should be persisted in registry")
	}

	// Existing entry should adopt its registered mode and not be new.
	gotB, includeB := upsertRepoIntoRegistry(reg, git.Repository{Path: pathB, Name: "b"}, now, git.ModeSyncDefault)
	if !includeB {
		t.Error("active entry should be included")
	}
	if gotB.IsNewRegistryEntry {
		t.Error("existing entry should not be marked new")
	}
	if gotB.Mode != git.ModeSyncAll {
		t.Errorf("existing entry should preserve registered mode (%v), got %v", git.ModeSyncAll, gotB.Mode)
	}

	// Ignored entry should be excluded.
	_, includeIgn := upsertRepoIntoRegistry(reg, git.Repository{Path: pathIgnored, Name: "ignored"}, now, git.ModeSyncDefault)
	if includeIgn {
		t.Error("ignored entry should not be included")
	}
	if e := reg.FindByPath(pathIgnored); e == nil || e.Status != registry.StatusIgnored {
		t.Error("ignored entry should remain ignored after upsert")
	}
}

func TestTruncateScanProgressPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		maxLen int
		want   string
	}{
		{"empty", "", 10, ""},
		{"shorter than max", "/a/b", 10, "/a/b"},
		{"equal to max", "/abcdefghi", 10, "/abcdefghi"},
		{"truncated with ellipsis", "/very/long/nested/path/to/repo", 12, "...h/to/repo"},
		{"maxLen too small for ellipsis", "/abc/def", 2, "/a"},
		{"maxLen=0 returns full path", "/keep", 0, "/keep"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateScanProgressPath(tt.path, tt.maxLen)
			if got != tt.want {
				t.Fatalf("truncateScanProgressPath(%q, %d) = %q, want %q", tt.path, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestScanProgressPathMaxLen(t *testing.T) {
	t.Run("no COLUMNS falls back", func(t *testing.T) {
		t.Setenv("COLUMNS", "")
		got := scanProgressPathMaxLen("prefix")
		if got != 72 {
			t.Fatalf("fallback expected 72, got %d", got)
		}
	})
	t.Run("dynamic shrinks below fallback", func(t *testing.T) {
		t.Setenv("COLUMNS", "40")
		got := scanProgressPathMaxLen("prefix-")
		// 40 - 7 = 33
		if got != 33 {
			t.Fatalf("dynamic want 33, got %d", got)
		}
	})
	t.Run("dynamic clamped to minLen", func(t *testing.T) {
		t.Setenv("COLUMNS", "5")
		got := scanProgressPathMaxLen("longprefix")
		if got != 8 {
			t.Fatalf("min-clamped want 8, got %d", got)
		}
	})
	t.Run("dynamic capped at fallback", func(t *testing.T) {
		t.Setenv("COLUMNS", "200")
		got := scanProgressPathMaxLen("p")
		if got != 72 {
			t.Fatalf("capped want 72, got %d", got)
		}
	})
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
