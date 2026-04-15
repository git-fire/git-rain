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
	rainFetchOnly       bool
	rainInit            bool
	rainConfigFile      string
	rainSelect          bool
	rainBranchMode      string
	rainSyncTags        bool
	forceUnlockRegistry bool
)

var rootCmd = &cobra.Command{
	Use:   "git-rain",
	Short: "Sync all local repos from their remotes",
	Long: `Git Rain — reverse sync from remotes

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
	rootCmd.Flags().BoolVar(&rainDryRun, "dry-run", false, "Show what would be synced without making changes")
	rootCmd.Flags().BoolVar(&rainFetchOnly, "fetch-only", false, "Run git fetch --all --prune per repo but skip local ref updates")
	rootCmd.Flags().BoolVar(&rainInit, "init", false, "Generate example ~/.config/git-rain/config.toml")
	rootCmd.Flags().StringVar(&rainConfigFile, "config", "", "Use an explicit config file path")
	rootCmd.Flags().BoolVar(&rainSelect, "select", false, "Interactive TUI repo selector before syncing")
	rootCmd.Flags().StringVar(&rainBranchMode, "branch-mode", "", `Branch sync mode: mainline (default), checked-out, all-local, all-branches`)
	rootCmd.Flags().BoolVar(&rainSyncTags, "tags", false, "Fetch all tags from remotes (default: off)")
	rootCmd.PersistentFlags().BoolVar(&forceUnlockRegistry, "force-unlock-registry", false, "Remove stale registry lock file without prompting")
}

func runRain(_ *cobra.Command, _ []string) error {
	if rainInit {
		return handleInit()
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
		MainlinePatterns: cfg.Global.MainlinePatterns,
	}

	if rainOpts.BranchMode == git.BranchSyncAllBranches {
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
		return runDryRun(opts, rainOpts)
	}

	if rainSelect {
		return runSelectStream(cfg, reg, regPath, opts, rainOpts)
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
	if riskyMode {
		fmt.Println("⚠️  Risky mode enabled: local-only commits may be realigned after backup branch creation")
	} else {
		fmt.Println("✓ Safe mode: local-only commits are preserved")
	}
	fmt.Println()

	totalUpdated := 0
	totalSkipped := 0
	totalFrozen := 0
	totalFailed := 0

	for _, repo := range active {
		fmt.Printf("  %s\n", repo.Name)

		if rainFetchOnly {
			if fetchErr := fetchOnly(repo.Path); fetchErr != nil {
				fmt.Printf("    ❄  (all branches): %s\n\n",
					safety.SanitizeText(fetchFailureMessage(fetchErr.Error())))
				totalFrozen++
				continue
			}
			fmt.Println("    ↓  fetched")
			fmt.Println()
			totalUpdated++
			continue
		}

		res, rainErr := git.RainRepository(repo.Path, rainOpts)
		if rainErr != nil {
			fmt.Printf("    ✗  failed: %s\n\n", safety.SanitizeText(rainErr.Error()))
			totalFailed++
			continue
		}
		if len(res.Branches) == 0 {
			fmt.Println("    ·  no local branches")
			fmt.Println()
			continue
		}

		for _, br := range res.Branches {
			symbol := weatherSymbol(br.Outcome)
			line := fmt.Sprintf("    %s  %s", symbol, br.Branch)
			if br.Upstream != "" {
				line += " ← " + br.Upstream
			}
			line += ": " + outcomeLabel(br.Outcome)
			if br.Message != "" {
				line += " — " + safety.SanitizeText(strings.TrimSpace(br.Message))
			}
			if br.BackupBranch != "" {
				line += " (backup: " + br.BackupBranch + ")"
			}
			fmt.Println(line)
		}
		fmt.Println()

		totalUpdated += res.Updated
		totalSkipped += res.Skipped
		totalFrozen += res.Frozen
		totalFailed += res.Failed
	}

	fmt.Println(strings.Repeat("─", 48))
	if totalFailed > 0 {
		fmt.Printf("🌧  rain stopped — %d updated, %d skipped, %d frozen, %d failed\n",
			totalUpdated, totalSkipped, totalFrozen, totalFailed)
		return fmt.Errorf("%d branch(es) failed — check output above", totalFailed)
	}
	if totalFrozen > 0 {
		fmt.Printf("🌧  rain delivered — %d updated, %d skipped, %d frozen (try again when reachable)\n",
			totalUpdated, totalSkipped, totalFrozen)
		return nil
	}
	fmt.Printf("🌧  rain delivered — %d updated, %d skipped\n", totalUpdated, totalSkipped)
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

func runDryRun(scanOpts git.ScanOptions, rainOpts git.RainOptions) error {
	repos, err := git.ScanRepositories(scanOpts)
	if err != nil {
		return fmt.Errorf("repository scan failed: %w", err)
	}

	fmt.Println("🌧️  Git Rain — dry run")
	if rainOpts.RiskyMode {
		fmt.Println("⚠️  Risky mode would be enabled")
	} else {
		fmt.Println("✓ Safe mode: local-only commits would be preserved")
	}
	fmt.Printf("Branch mode: %s\n", rainOpts.BranchMode)
	fmt.Println()

	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return nil
	}

	fmt.Printf("Would hydrate %d repositor", len(repos))
	if len(repos) == 1 {
		fmt.Println("y:")
	} else {
		fmt.Println("ies:")
	}
	for _, repo := range repos {
		fmt.Printf("  • %s (%s)\n", repo.Name, repo.Path)
	}
	return nil
}

// runSelectStream launches the interactive TUI repo selector (streaming mode).
// The filesystem scan runs concurrently; repos stream into the selector as they
// are discovered. After selection, rain runs on the chosen repos.
func runSelectStream(cfg *config.Config, reg *registry.Registry, regPath string, opts git.ScanOptions, rainOpts git.RainOptions) error {
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

	return runRainOnRepos(selected, rainOpts)
}

// runRainOnRepos runs the rain operation on a pre-selected list of repos.
func runRainOnRepos(repos []git.Repository, opts git.RainOptions) error {
	fmt.Println("🌧️  Git Rain")
	if opts.RiskyMode {
		fmt.Println("⚠️  Risky mode enabled: local-only commits may be realigned after backup branch creation")
	} else {
		fmt.Println("✓ Safe mode: local-only commits are preserved")
	}
	fmt.Println()

	totalUpdated := 0
	totalSkipped := 0
	totalFrozen := 0
	totalFailed := 0

	for _, repo := range repos {
		fmt.Printf("  %s\n", repo.Name)

		if rainFetchOnly {
			if fetchErr := fetchOnly(repo.Path); fetchErr != nil {
				fmt.Printf("    ❄  (all branches): %s\n\n",
					safety.SanitizeText(fetchFailureMessage(fetchErr.Error())))
				totalFrozen++
				continue
			}
			fmt.Println("    ↓  fetched")
			fmt.Println()
			totalUpdated++
			continue
		}

		res, rainErr := git.RainRepository(repo.Path, opts)
		if rainErr != nil {
			fmt.Printf("    ✗  failed: %s\n\n", safety.SanitizeText(rainErr.Error()))
			totalFailed++
			continue
		}
		if len(res.Branches) == 0 {
			fmt.Println("    ·  no local branches")
			fmt.Println()
			continue
		}

		for _, br := range res.Branches {
			symbol := weatherSymbol(br.Outcome)
			line := fmt.Sprintf("    %s  %s", symbol, br.Branch)
			if br.Upstream != "" {
				line += " ← " + br.Upstream
			}
			line += ": " + outcomeLabel(br.Outcome)
			if br.Message != "" {
				line += " — " + safety.SanitizeText(strings.TrimSpace(br.Message))
			}
			if br.BackupBranch != "" {
				line += " (backup: " + br.BackupBranch + ")"
			}
			fmt.Println(line)
		}
		fmt.Println()

		totalUpdated += res.Updated
		totalSkipped += res.Skipped
		totalFrozen += res.Frozen
		totalFailed += res.Failed
	}

	fmt.Println(strings.Repeat("─", 48))
	if totalFailed > 0 {
		fmt.Printf("🌧  rain stopped — %d updated, %d skipped, %d frozen, %d failed\n",
			totalUpdated, totalSkipped, totalFrozen, totalFailed)
		return fmt.Errorf("%d branch(es) failed — check output above", totalFailed)
	}
	if totalFrozen > 0 {
		fmt.Printf("🌧  rain delivered — %d updated, %d skipped, %d frozen (try again when reachable)\n",
			totalUpdated, totalSkipped, totalFrozen)
		return nil
	}
	fmt.Printf("🌧  rain delivered — %d updated, %d skipped\n", totalUpdated, totalSkipped)
	return nil
}

func fetchOnly(repoPath string) error {
	cmd := exec.Command("git", "fetch", "--all", "--prune")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch --all --prune: %w (output: %s)", err, strings.TrimSpace(string(out)))
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
