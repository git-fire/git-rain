// Package cmd implements the git-rain Cobra CLI.
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/git-rain/git-rain/internal/config"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
	"github.com/git-rain/git-rain/internal/safety"
	"github.com/git-rain/git-rain/internal/ui"
)

// Version is set at build time via -ldflags "-X github.com/git-rain/git-rain/cmd.Version=vX.Y.Z"
var Version = "dev"

var (
	rainPath            string
	rainNoScan          bool
	rainRisky           bool
	rainDryRun          bool
	rainFetchMainline   bool
	rainInit            bool
	rainConfigFile      string
	rainRain            bool
	rainSync            bool
	rainBranchMode      string
	rainSyncTags        bool
	rainPrune           bool
	rainNoPrune         bool
	forceUnlockRegistry bool
)

var rootCmd = &cobra.Command{
	Use:   "git-rain",
	Short: "Fetch all remotes by default; narrower fetches and --sync hydrate locals",
	Long: `Git Rain — reverse sync from remotes

Modes (from lightest fetch to full local updates):

  1. Default — git fetch --all per repo (all remote-tracking refs; local branch
     refs stay put). --prune is opt-in (destructive to stale remote-tracking refs);
     use --prune, config fetch_prune, registry fetch_prune, or git config rain.fetchprune.

  2. --fetch-mainline — targeted fetches for mainline branches only (faster when
     you do not need every remote ref). Cannot be combined with --sync, --risky,
     non-mainline --branch-mode, or global risky_mode (full-sync triggers).

  3. --sync — hydrate local branches from remotes. Use --branch-mode for scope
     (mainline, checked-out, all-local, all-branches); all-branches can create
     many local branches from the remote.

  4. --risky — with --sync (or config), allows destructive realignment of
     local-only commits after creating git-rain-backup-* refs.

Safety-first defaults:
  - never rewrites local-only commits
  - fast-forwards current branch only when merge can be done safely
  - updates non-checked-out branch refs directly

Risky mode (config: global.risky_mode, flag: --risky) allows destructive
realignment of local-only commits after creating git-rain-backup-* refs.`,
	RunE: runRain,
}

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	v := Version
	if v == "" {
		v = "dev"
	}
	rootCmd.Version = v
	rootCmd.SilenceUsage = true
	rootCmd.Flags().StringVar(&rainPath, "path", "", "Path to scan for repositories (default: config global.scan_path)")
	rootCmd.Flags().BoolVar(&rainNoScan, "no-scan", false, "Skip filesystem scan; hydrate only known registry repos")
	rootCmd.Flags().BoolVar(&rainRisky, "risky", false, "Allow destructive local branch realignment after creating backup refs")
	rootCmd.Flags().BoolVar(&rainDryRun, "dry-run", false, "Show what would run without making changes")
	rootCmd.Flags().BoolVar(&rainFetchMainline, "fetch-mainline", false, "Fetch only mainline remote-tracking refs per remote (lighter than the default git fetch --all)")
	rootCmd.Flags().BoolVar(&rainInit, "init", false, "Generate example ~/.config/git-rain/config.toml")
	rootCmd.Flags().StringVar(&rainConfigFile, "config", "", "Use an explicit config file path")
	rootCmd.Flags().BoolVar(&rainRain, "rain", false, "Interactive TUI repo picker before running (mirrors git-fire --fire)")
	rootCmd.Flags().BoolVar(&rainSync, "sync", false, "Update local branches from remotes (default is git fetch --all only)")
	rootCmd.Flags().StringVar(&rainBranchMode, "branch-mode", "", `Branch sync mode: mainline (default), checked-out, all-local, all-branches`)
	rootCmd.Flags().BoolVar(&rainSyncTags, "tags", false, "Fetch all tags from remotes (default: off)")
	rootCmd.Flags().BoolVar(&rainPrune, "prune", false, "Pass --prune on git fetch for this run (overrides config and per-repo settings)")
	rootCmd.Flags().BoolVar(&rainNoPrune, "no-prune", false, "Never pass --prune on git fetch for this run (overrides --prune and global/repo enablement)")
	rootCmd.PersistentFlags().BoolVar(&forceUnlockRegistry, "force-unlock-registry", false, "Remove stale registry lock file without prompting")
}

// computeFullSync decides whether to run full branch hydration (RainRepository)
// versus fetch-only paths (default full fetch or --fetch-mainline).
func computeFullSync(explicitSync, riskyFlag, riskyCfg bool, branchFlag, branchCfg string) bool {
	if explicitSync || riskyFlag || riskyCfg {
		return true
	}
	if branchFlag != "" {
		return true
	}
	return git.ParseBranchSyncMode(branchCfg) != git.BranchSyncMainline
}

func runRain(_ *cobra.Command, _ []string) error {
	if rainInit {
		return handleInit()
	}
	if rainPrune && rainNoPrune {
		return fmt.Errorf("cannot use --prune and --no-prune together")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: please install git before using git-rain")
	}

	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigFile: rainConfigFile})
	if err != nil {
		return fmt.Errorf("failed to load config: %s", safety.SanitizeText(err.Error()))
	}
	if rainPath != "" {
		cfg.Global.ScanPath = rainPath
	}
	if rainNoScan {
		cfg.Global.DisableScan = true
	}
	riskyMode := cfg.Global.RiskyMode || rainRisky

	branchModeStr := cfg.Global.BranchMode
	if rainBranchMode != "" {
		branchModeStr = rainBranchMode
	}
	rainOpts := git.RainOptions{
		RiskyMode:        riskyMode,
		BranchMode:       git.ParseBranchSyncMode(branchModeStr),
		SyncTags:         cfg.Global.SyncTags || rainSyncTags,
		FetchPrune:       cfg.Global.FetchPrune,
		MainlinePatterns: cfg.Global.MainlinePatterns,
	}

	fullSync := computeFullSync(rainSync, rainRisky, cfg.Global.RiskyMode, rainBranchMode, cfg.Global.BranchMode)
	if fullSync && rainFetchMainline {
		return fmt.Errorf("--fetch-mainline only applies to fetch-only runs; remove --sync, --risky, non-mainline --branch-mode, or risky_mode in config")
	}
	if fullSync && rainOpts.BranchMode == git.BranchSyncAllBranches {
		fmt.Println("⚠️  all-branches mode: remote branches will be created locally — this can produce many local branches")
	}

	reg := &registry.Registry{}
	regPath := ""
	if p, pathErr := registry.DefaultRegistryPath(); pathErr != nil {
		fmt.Fprintf(os.Stderr, "warning: registry disabled: %v\n", pathErr)
	} else if unlockErr := maybeOfferRegistryUnlock(p); unlockErr != nil {
		return unlockErr
	} else if loaded, loadErr := registry.Load(p); loadErr != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring unreadable registry %s: %v\n", p, loadErr)
	} else {
		regPath = p
		reg = loaded
	}

	for i, entry := range reg.Repos {
		if entry.Status == registry.StatusIgnored {
			continue
		}
		if _, statErr := os.Stat(entry.Path); statErr != nil {
			if os.IsNotExist(statErr) && reg.Repos[i].Status != registry.StatusMissing {
				reg.Repos[i].Status = registry.StatusMissing
			}
			continue
		}
		if entry.Status == registry.StatusMissing || entry.Status == "" {
			reg.Repos[i].Status = registry.StatusActive
		}
	}

	opts := git.DefaultScanOptions()
	opts.RootPath = cfg.Global.ScanPath
	opts.Exclude = cfg.Global.ScanExclude
	opts.MaxDepth = cfg.Global.ScanDepth
	opts.Workers = cfg.Global.ScanWorkers
	opts.KnownPaths = buildKnownPaths(reg, cfg.Global.RescanSubmodules)
	opts.DisableScan = cfg.Global.DisableScan

	if opts.DisableScan {
		fmt.Println("⚠️  Rain scanning disabled: hydrating known registry repositories only")
	}

	if rainDryRun {
		return runDryRun(opts, rainOpts, fullSync, cfg)
	}

	if rainRain {
		return runRainTUIStream(cfg, reg, regPath, opts, rainOpts, fullSync)
	}

	repos, err := git.ScanRepositories(opts)
	if err != nil {
		return fmt.Errorf("repository scan failed: %w", err)
	}

	now := time.Now()
	defaultMode := git.ParseMode(cfg.Global.DefaultMode)
	for i, repo := range repos {
		repos[i], _ = upsertRepoIntoRegistry(reg, repo, now, defaultMode)
	}
	saveRegistry(reg, regPath)

	active := make([]git.Repository, 0, len(repos))
	for _, repo := range repos {
		absPath, absErr := filepath.Abs(repo.Path)
		if absErr != nil {
			active = append(active, repo)
			continue
		}
		entry := reg.FindByPath(absPath)
		if entry != nil && entry.Status == registry.StatusIgnored {
			continue
		}
		active = append(active, repo)
	}
	if len(active) == 0 {
		fmt.Println("No git repositories found.")
		return nil
	}

	fmt.Println("🌧️  Git Rain")
	if fullSync {
		if riskyMode {
			fmt.Println("⚠️  Risky mode enabled: local-only commits may be realigned after backup branch creation")
		} else {
			fmt.Println("✓ Safe mode: local-only commits are preserved")
		}
	} else if rainFetchMainline {
		fmt.Println("✓ Mainline fetch: mainline remote-tracking refs only (--fetch-mainline; --prune is opt-in)")
	} else {
		fmt.Println("✓ Default: git fetch --all per repo (remote-tracking refs only; --prune opt-in; --fetch-mainline for less; --sync to update locals)")
	}
	fmt.Println()

	totals := rainTotals{}
	processRainRepositories(reg, active, rainOpts, fullSync, &totals)

	fmt.Println(strings.Repeat("─", 48))
	if totals.failed > 0 {
		fmt.Printf("🌧  rain stopped — %d updated, %d skipped, %d frozen, %d failed\n",
			totals.updated, totals.skipped, totals.frozen, totals.failed)
		return fmt.Errorf("%d branch(es) failed — check output above", totals.failed)
	}
	if totals.frozen > 0 {
		fmt.Printf("🌧  rain delivered — %d updated, %d skipped, %d frozen (try again when reachable)\n",
			totals.updated, totals.skipped, totals.frozen)
		return nil
	}
	fmt.Printf("🌧  rain delivered — %d updated, %d skipped\n", totals.updated, totals.skipped)
	return nil
}

// weatherSymbol returns the terminal symbol for a branch outcome.
func weatherSymbol(outcome string) string {
	switch outcome {
	case git.RainOutcomeUpdated:
		return "↓"
	case git.RainOutcomeUpdatedRisky:
		return "⚡"
	case git.RainOutcomeUpToDate:
		return "·"
	case git.RainOutcomeFetched:
		return "↓"
	case git.RainOutcomeFrozen:
		return "❄"
	case git.RainOutcomeFailed:
		return "✗"
	default:
		return "~" // fog — skipped for any local-state reason
	}
}

// outcomeLabel returns a calm, readable label for a branch outcome.
func outcomeLabel(outcome string) string {
	switch outcome {
	case git.RainOutcomeUpdated:
		return "synced"
	case git.RainOutcomeUpdatedRisky:
		return "realigned"
	case git.RainOutcomeUpToDate:
		return "current"
	case git.RainOutcomeFetched:
		return "fetched"
	case git.RainOutcomeFrozen:
		return "frozen"
	case git.RainOutcomeFailed:
		return "failed"
	case git.RainOutcomeSkippedNoUpstream:
		return "no upstream"
	case git.RainOutcomeSkippedAmbiguousUpstream:
		return "ambiguous upstream"
	case git.RainOutcomeSkippedUpstreamMissing:
		return "upstream missing"
	case git.RainOutcomeSkippedCheckedOut:
		return "checked out elsewhere"
	case git.RainOutcomeSkippedLocalAhead:
		return "local ahead"
	case git.RainOutcomeSkippedDiverged:
		return "diverged"
	case git.RainOutcomeSkippedUnsafeMerge:
		return "unsafe merge"
	case git.RainOutcomeSkippedUnsafeDirty:
		return "dirty worktree"
	default:
		return outcome
	}
}

// printRainBranchResults prints one line per branch (mainline fetch or full sync).
func printRainBranchResults(branches []git.RainBranchResult, showBackup bool) {
	for _, br := range branches {
		symbol := weatherSymbol(br.Outcome)
		line := fmt.Sprintf("    %s  %s", symbol, br.Branch)
		if br.Upstream != "" {
			line += " ← " + br.Upstream
		}
		line += ": " + outcomeLabel(br.Outcome)
		if br.Message != "" {
			line += " — " + safety.SanitizeText(strings.TrimSpace(br.Message))
		}
		if showBackup && br.BackupBranch != "" {
			line += " (backup: " + br.BackupBranch + ")"
		}
		fmt.Println(line)
	}
}

// fetchFailureMessage extracts a calm message from a fetchOnly error.
func fetchFailureMessage(errMsg string) string {
	s := strings.ToLower(errMsg)
	if strings.Contains(s, "authentication") || strings.Contains(s, "permission denied") ||
		strings.Contains(s, "could not read") || strings.Contains(s, "401") || strings.Contains(s, "403") {
		return "could not authenticate with remote — check your credentials and try again"
	}
	if strings.Contains(s, "could not resolve") || strings.Contains(s, "connection") ||
		strings.Contains(s, "timed out") || strings.Contains(s, "network") {
		return "could not reach remote — check your network and try again"
	}
	return "fetch did not complete — try again when the remote is reachable"
}

type rainTotals struct {
	updated int
	skipped int
	frozen  int
	failed  int
}

// processRainRepositories runs fetch or full rain for each repository and accumulates outcome counts.
func processRainRepositories(reg *registry.Registry, repos []git.Repository, rainOpts git.RainOptions, fullSync bool, totals *rainTotals) {
	for _, repo := range repos {
		fmt.Printf("  %s\n", repo.Name)

		absRepo, absErr := filepath.Abs(repo.Path)
		if absErr != nil {
			absRepo = repo.Path
		}
		repoOpts, pruneErr := applyRepoFetchPrune(repo.Path, rainOpts, absRepo, reg)
		if pruneErr != nil {
			fmt.Printf("    ✗  failed: %s\n\n", safety.SanitizeText(pruneErr.Error()))
			totals.failed++
			continue
		}

		if !fullSync && !rainFetchMainline {
			if fetchErr := fetchOnly(repo.Path, repoOpts); fetchErr != nil {
				fmt.Printf("    ❄  (fetch --all): %s\n\n",
					safety.SanitizeText(fetchFailureMessage(fetchErr.Error())))
				totals.frozen++
				continue
			}
			fmt.Println("    ↓  fetched")
			fmt.Println()
			totals.updated++
			continue
		}

		if !fullSync {
			res, fetchErr := git.MainlineFetchRemotes(repo.Path, repoOpts)
			if fetchErr != nil {
				fmt.Printf("    ✗  failed: %s\n\n", safety.SanitizeText(fetchErr.Error()))
				totals.failed++
				continue
			}
			if len(res.Branches) == 0 {
				fmt.Println("    ·  no mainline branches to fetch")
				fmt.Println()
				continue
			}
			printRainBranchResults(res.Branches, false)
			fmt.Println()
			totals.updated += res.Updated
			totals.skipped += res.Skipped
			totals.frozen += res.Frozen
			totals.failed += res.Failed
			continue
		}

		res, rainErr := git.RainRepository(repo.Path, repoOpts)
		if rainErr != nil {
			fmt.Printf("    ✗  failed: %s\n\n", safety.SanitizeText(rainErr.Error()))
			totals.failed++
			continue
		}
		if len(res.Branches) == 0 {
			fmt.Println("    ·  no local branches")
			fmt.Println()
			continue
		}

		printRainBranchResults(res.Branches, true)
		fmt.Println()

		totals.updated += res.Updated
		totals.skipped += res.Skipped
		totals.frozen += res.Frozen
		totals.failed += res.Failed
	}
}

func runDryRun(scanOpts git.ScanOptions, rainOpts git.RainOptions, fullSync bool, cfg *config.Config) error {
	repos, err := git.ScanRepositories(scanOpts)
	if err != nil {
		return fmt.Errorf("repository scan failed: %w", err)
	}

	fmt.Println("🌧️  Git Rain — dry run")
	if fullSync {
		if rainOpts.RiskyMode {
			fmt.Println("⚠️  Risky mode would be enabled")
		} else {
			fmt.Println("✓ Safe mode: local-only commits would be preserved")
		}
		fmt.Printf("Branch mode: %s\n", rainOpts.BranchMode)
	} else if rainFetchMainline {
		fmt.Println("✓ Dry run would fetch mainline remote-tracking refs only (not full git fetch --all)")
	} else {
		fmt.Println("✓ Default dry run would run git fetch --all per repo (not --sync)")
	}
	switch {
	case rainNoPrune:
		fmt.Println("✓ Fetch --prune: off for this run (--no-prune)")
	case rainPrune:
		fmt.Println("✓ Fetch --prune: on for this run (--prune)")
	case cfg.Global.FetchPrune:
		fmt.Println("✓ Fetch --prune: global default on (fetch_prune); per-repo git config rain.fetchprune or registry fetch_prune can override")
	default:
		fmt.Println("✓ Fetch --prune: off unless enabled per repo (git config rain.fetchprune), registry fetch_prune, global fetch_prune, or --prune")
	}
	fmt.Println()

	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return nil
	}

	if fullSync {
		fmt.Printf("Would hydrate %d repositor", len(repos))
		if len(repos) == 1 {
			fmt.Println("y:")
		} else {
			fmt.Println("ies:")
		}
	} else if rainFetchMainline {
		fmt.Printf("Would fetch mainline remote-tracking refs in %d repositor", len(repos))
		if len(repos) == 1 {
			fmt.Println("y:")
		} else {
			fmt.Println("ies:")
		}
	} else {
		fmt.Printf("Would run git fetch --all in %d repositor", len(repos))
		if len(repos) == 1 {
			fmt.Println("y:")
		} else {
			fmt.Println("ies:")
		}
	}
	for _, repo := range repos {
		fmt.Printf("  • %s (%s)\n", repo.Name, repo.Path)
	}
	return nil
}

// runRainTUIStream launches the interactive TUI repo picker (streaming mode).
// The filesystem scan runs concurrently; repos stream into the picker as they
// are discovered. After confirmation, the same default or --sync behavior runs.
func runRainTUIStream(cfg *config.Config, reg *registry.Registry, regPath string, opts git.ScanOptions, rainOpts git.RainOptions, fullSync bool) error {
	ctx, cancelScan := context.WithCancel(context.Background())
	defer cancelScan()
	opts.Ctx = ctx

	folderProgress := make(chan string, 32)
	opts.FolderProgress = folderProgress

	scanChan := make(chan git.Repository, opts.Workers)
	tuiRepoChan := make(chan git.Repository, opts.Workers)

	var scanErr error
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		scanErr = git.ScanRepositoriesStream(opts, scanChan)
	}()

	now := time.Now()
	defaultMode := git.ParseMode(cfg.Global.DefaultMode)
	go func() {
		defer close(tuiRepoChan)
		for repo := range scanChan {
			repo, include := upsertRepoIntoRegistry(reg, repo, now, defaultMode)
			if include {
				repo.Selected = true
				tuiRepoChan <- repo
			}
		}
		saveRegistry(reg, regPath)
	}()

	userCfgDir, _ := config.UserGitRainDir()
	cfgPath := filepath.Join(userCfgDir, "config.toml")

	selected, err := ui.RunRepoSelectorStream(
		tuiRepoChan,
		folderProgress,
		cfg.Global.DisableScan,
		rainNoScan,
		cfg,
		cfgPath,
		reg,
		regPath,
	)

	// Drain channels before cancelling so goroutines can't block on sends.
	go func() {
		for range tuiRepoChan {
		}
	}()
	go func() {
		for range folderProgress {
		}
	}()
	cancelScan()
	<-scanDone

	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		return fmt.Errorf("repo selection failed: %w", err)
	}
	if len(selected) == 0 {
		fmt.Println("No repositories selected.")
		return nil
	}

	if scanErr != nil {
		fmt.Fprintf(os.Stderr, "warning: scan error: %s\n", safety.SanitizeText(scanErr.Error()))
	}

	fmt.Println("Selected repositories:")
	for _, repo := range selected {
		dirty := ""
		if repo.IsDirty {
			dirty = " (dirty)"
		}
		fmt.Printf("  • %s%s\n", repo.Name, dirty)
	}
	fmt.Println()

	return runRainOnRepos(reg, selected, rainOpts, fullSync)
}

// runRainOnRepos runs fetch or full rain on a pre-selected list of repos.
func runRainOnRepos(reg *registry.Registry, repos []git.Repository, opts git.RainOptions, fullSync bool) error {
	fmt.Println("🌧️  Git Rain")
	if fullSync {
		if opts.RiskyMode {
			fmt.Println("⚠️  Risky mode enabled: local-only commits may be realigned after backup branch creation")
		} else {
			fmt.Println("✓ Safe mode: local-only commits are preserved")
		}
	} else if rainFetchMainline {
		fmt.Println("✓ Mainline fetch: mainline remote-tracking refs only (--fetch-mainline; --prune is opt-in)")
	} else {
		fmt.Println("✓ Default: git fetch --all per repo (use --fetch-mainline for less; --sync to update locals; --prune opt-in)")
	}
	fmt.Println()

	totals := rainTotals{}
	processRainRepositories(reg, repos, opts, fullSync, &totals)

	fmt.Println(strings.Repeat("─", 48))
	if totals.failed > 0 {
		fmt.Printf("🌧  rain stopped — %d updated, %d skipped, %d frozen, %d failed\n",
			totals.updated, totals.skipped, totals.frozen, totals.failed)
		return fmt.Errorf("%d branch(es) failed — check output above", totals.failed)
	}
	if totals.frozen > 0 {
		fmt.Printf("🌧  rain delivered — %d updated, %d skipped, %d frozen (try again when reachable)\n",
			totals.updated, totals.skipped, totals.frozen)
		return nil
	}
	fmt.Printf("🌧  rain delivered — %d updated, %d skipped\n", totals.updated, totals.skipped)
	return nil
}

func applyRepoFetchPrune(repoPath string, base git.RainOptions, absPath string, reg *registry.Registry) (git.RainOptions, error) {
	opt := base
	cliSet := rainPrune || rainNoPrune
	cliVal := rainPrune && !rainNoPrune
	if cliSet {
		opt.FetchPrune = git.ResolveFetchPrune(cliSet, cliVal, false, false, nil, base.FetchPrune)
		return opt, nil
	}
	var regPrune *bool
	if reg != nil {
		if e := reg.FindByPath(absPath); e != nil {
			regPrune = e.FetchPrune
		}
	}
	gitSet, gitVal, err := git.ReadRainFetchPrune(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: reading %s in %s: %v\n", git.RainFetchPruneConfigKey, repoPath, err)
		gitSet, gitVal = false, false
	}
	opt.FetchPrune = git.ResolveFetchPrune(cliSet, cliVal, gitSet, gitVal, regPrune, base.FetchPrune)
	return opt, nil
}

func fetchOnly(repoPath string, opts git.RainOptions) error {
	fetchArgs := []string{"fetch", "--all"}
	if opts.FetchPrune {
		fetchArgs = append(fetchArgs, "--prune")
	}
	if opts.SyncTags {
		fetchArgs = append(fetchArgs, "--tags")
	}
	cmd := exec.Command("git", fetchArgs...)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w (output: %s)", strings.Join(append([]string{"git"}, fetchArgs...), " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func handleInit() error {
	cfgPath := config.DefaultConfigPath()
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Fprintf(os.Stderr, "Config file already exists: %s\n", cfgPath)
		fmt.Fprintf(os.Stderr, "Use --force to overwrite.\n")
		return fmt.Errorf("config file already exists: %s", cfgPath)
	}
	if err := config.WriteExampleConfig(cfgPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Printf("Created example config: %s\n", cfgPath)
	return nil
}

func maybeOfferRegistryUnlock(regPath string) error {
	if regPath == "" {
		return nil
	}

	if forceUnlockRegistry {
		lockPath := registry.LockPath(regPath)
		if _, err := os.Stat(lockPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("registry lock: %w", err)
		}
		info, _ := registry.ReadLockFile(regPath)
		fmt.Fprintf(os.Stderr, "warning: removing registry lock file (--force-unlock-registry): %s\n", lockPath)
		if info != nil {
			if info.OwnerAppearsAlive && info.PID > 0 {
				fmt.Fprintf(os.Stderr, "warning: lock listed PID %d, which still appears to be running\n", info.PID)
			} else if info.PID > 0 {
				fmt.Fprintf(os.Stderr, "warning: lock listed PID %d; only remove this if no other git-rain is running\n", info.PID)
			}
		}
		if err := registry.RemoveLockFile(regPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing registry lock: %w", err)
		}
		return nil
	}

	info, err := registry.ReadLockFile(regPath)
	if err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	if info == nil {
		return nil
	}

	if !info.OwnerAppearsAlive {
		fmt.Fprintf(os.Stderr, "warning: removing stale registry lock (owner process is gone): %s\n", info.LockPath)
		if err := registry.RemoveLockFile(regPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing stale registry lock: %w", err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "\nRegistry lock file is present:\n  %s\n", info.LockPath)
	fmt.Fprintf(os.Stderr, "This usually means another git-rain is running, or a previous run exited uncleanly.\n")
	if info.PID > 0 {
		fmt.Fprintf(os.Stderr, "Lock owner PID: %d (still appears to be running).\n", info.PID)
	}
	fmt.Fprintf(os.Stderr, "\nRemoving the lock while another instance is active can corrupt your repo registry.\n")
	fmt.Fprintf(os.Stderr, "If you are sure no other git-rain is running, you can remove the lock and continue.\n\n")

	if stat, err := os.Stdin.Stat(); err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		return fmt.Errorf("registry is locked; pass --force-unlock-registry to remove %s non-interactively", info.LockPath)
	}

	fmt.Print("Remove lock and continue? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		if err := registry.RemoveLockFile(regPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing registry lock: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Lock removed.")
		return nil
	default:
		return fmt.Errorf("aborted: registry lock still present at %s", safety.SanitizeText(info.LockPath))
	}
}

func buildKnownPaths(reg *registry.Registry, globalRescan bool) map[string]bool {
	m := make(map[string]bool, len(reg.Repos))
	for _, e := range reg.Repos {
		if e.Status == registry.StatusIgnored {
			continue
		}
		if e.Status != registry.StatusActive && e.Status != registry.StatusMissing && e.Status != "" {
			continue
		}
		abs, err := filepath.Abs(e.Path)
		if err != nil {
			continue
		}
		rescan := globalRescan
		if e.RescanSubmodules != nil {
			rescan = *e.RescanSubmodules
		}
		m[abs] = rescan
	}
	return m
}

func upsertRepoIntoRegistry(reg *registry.Registry, repo git.Repository, now time.Time, defaultMode git.RepoMode) (git.Repository, bool) {
	absPath, err := filepath.Abs(repo.Path)
	if err != nil {
		repo.IsNewRegistryEntry = false
		return repo, true
	}
	var modeStr string
	var ignored bool
	found := reg.UpdateByPath(absPath, func(e *registry.RegistryEntry) {
		modeStr = e.Mode
		e.LastSeen = now
		if e.Status == registry.StatusIgnored {
			ignored = true
			return
		}
		e.Status = registry.StatusActive
	})
	if found {
		repo.IsNewRegistryEntry = false
		if modeStr != "" {
			repo.Mode = git.ParseMode(modeStr)
		} else {
			repo.Mode = defaultMode
		}
		return repo, !ignored
	}
	repo.Mode = defaultMode
	repo.IsNewRegistryEntry = true
	reg.Upsert(registry.RegistryEntry{
		Path:     absPath,
		Name:     repo.Name,
		Status:   registry.StatusActive,
		Mode:     repo.Mode.String(),
		AddedAt:  now,
		LastSeen: now,
	})
	return repo, true
}

func saveRegistry(reg *registry.Registry, regPath string) {
	if regPath == "" {
		return
	}
	if err := registry.Save(reg, regPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save registry: %v\n", err)
	}
}
