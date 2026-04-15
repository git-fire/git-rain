package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// mainlineBranchNames are exact branch names considered mainline.
var mainlineBranchNames = map[string]bool{
	"main":    true,
	"master":  true,
	"trunk":   true,
	"develop": true,
	"dev":     true,
}

// mainlineGitflowPrefixes are branch name prefixes considered mainline.
var mainlineGitflowPrefixes = []string{
	"release/",
	"hotfix/",
	"support/",
}

// matchesPatterns reports whether name matches any user-defined pattern.
// Each pattern is tested as both an exact match and a prefix, so "JIRA-"
// matches "JIRA-123" and "feat/" matches "feat/new-thing". Patterns ending
// in "/" are prefix-only (never an exact branch name).
func matchesPatterns(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if name == p || strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// fetchFailureReason classifies a failed git-fetch stderr output into a
// human-friendly frozen reason. Returns a calm, non-alarming message.
func fetchFailureReason(output []byte) string {
	s := strings.ToLower(string(output))
	authPhrases := []string{
		"authentication failed",
		"could not read username",
		"could not read password",
		"permission denied",
		"repository not found",
		"terminal prompts disabled",
		"invalid username or password",
		"invalid credentials",
		" 403 ", " 401 ",
	}
	for _, p := range authPhrases {
		if strings.Contains(s, p) {
			return "could not authenticate with remote — check your credentials and try again"
		}
	}
	netPhrases := []string{
		"could not resolve host",
		"connection refused",
		"network is unreachable",
		"timed out",
		"connection timed out",
		"no route to host",
		"temporary failure in name resolution",
	}
	for _, p := range netPhrases {
		if strings.Contains(s, p) {
			return "could not reach remote — check your network and try again"
		}
	}
	return "fetch did not complete — try again when the remote is reachable"
}

// isMainlineBranch reports whether a branch name is a mainline or gitflow branch.
func isMainlineBranch(name string) bool {
	if mainlineBranchNames[name] {
		return true
	}
	for _, prefix := range mainlineGitflowPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// RainOptions controls how rain/hydrate updates local branches from remotes.
type RainOptions struct {
	// RiskyMode allows destructive realignment of local-only commits after
	// creating a backup branch.
	RiskyMode bool

	// BranchMode controls which branches are eligible for syncing.
	// Defaults to BranchSyncMainline when zero.
	BranchMode BranchSyncMode

	// SyncTags fetches all tags from the remote in addition to branch refs.
	// Off by default because tag fetches can pull large or unwanted history.
	SyncTags bool

	// MainlinePatterns extends the built-in mainline branch list with user-defined
	// patterns. Each entry is either an exact branch name or a prefix ending in "/"
	// (e.g. "feat/", "JIRA-"). Only used when BranchMode == BranchSyncMainline.
	MainlinePatterns []string
}

const (
	// RainOutcomeUpdated — branch fast-forwarded from remote. ↓ Rain delivered.
	RainOutcomeUpdated = "updated"
	// RainOutcomeUpdatedRisky — branch realigned via hard reset after backup. ⚡ Lightning.
	RainOutcomeUpdatedRisky = "updated-risky"
	// RainOutcomeUpToDate — local already matches upstream. · Nothing to do.
	RainOutcomeUpToDate = "up-to-date"
	// Skipped variants — fog. Local state prevents update; no changes made.
	RainOutcomeSkippedNoUpstream        = "skipped-no-upstream"
	RainOutcomeSkippedAmbiguousUpstream = "skipped-ambiguous-upstream"
	RainOutcomeSkippedUpstreamMissing   = "skipped-upstream-missing"
	RainOutcomeSkippedCheckedOut        = "skipped-checked-out"
	RainOutcomeSkippedLocalAhead        = "skipped-local-ahead"
	RainOutcomeSkippedDiverged          = "skipped-diverged"
	RainOutcomeSkippedUnsafeMerge       = "skipped-unsafe-merge"
	RainOutcomeSkippedUnsafeDirty       = "skipped-unsafe-dirty"
	// RainOutcomeFrozen — fetch failed (auth, network). ❄ Frozen in place, try again later.
	RainOutcomeFrozen = "frozen"
	// RainOutcomeFailed — hard git error during local branch operation. ✗ Ice.
	RainOutcomeFailed = "failed"
)

// RainBranchResult reports the outcome for one local branch.
type RainBranchResult struct {
	Branch       string
	Upstream     string
	Current      bool
	Outcome      string
	Message      string
	BackupBranch string
}

// RainResult reports per-repo rain/hydrate outcomes.
type RainResult struct {
	RepoPath string
	Branches []RainBranchResult
	Updated  int
	Skipped  int
	Frozen   int // fetch failed (auth/network) — NBD, try again later
	Failed   int // hard local git error
}

type localBranchTracking struct {
	Branch   string
	Upstream string
	Current  bool
}

// RainRepository updates local branches toward their upstream refs while
// preserving worktree/index safety by default.
func RainRepository(repoPath string, opts RainOptions) (RainResult, error) {
	result := RainResult{
		RepoPath: repoPath,
		Branches: make([]RainBranchResult, 0),
	}

	branches, err := listLocalBranchesWithUpstream(repoPath)
	if err != nil {
		return result, err
	}
	if len(branches) == 0 {
		return result, nil
	}

	// Refresh remote-tracking refs before we compare branch ancestry.
	hasRemote, err := repoHasAnyRemote(repoPath)
	if err != nil {
		return result, err
	}
	if hasRemote {
		fetchArgs := []string{"fetch", "--all", "--prune"}
		if opts.SyncTags {
			fetchArgs = append(fetchArgs, "--tags")
		}
		cmd := exec.Command("git", fetchArgs...)
		cmd.Dir = repoPath
		if output, fetchErr := cmd.CombinedOutput(); fetchErr != nil {
			// Freeze gracefully — could not reach remote (auth, network, etc.).
			// This is not a hard failure; the repo is untouched. Try again later.
			reason := fetchFailureReason(output)
			candidates := branches
			if len(candidates) == 0 {
				candidates = []localBranchTracking{{Branch: "(all branches)"}}
			}
			for _, b := range candidates {
				result.Branches = append(result.Branches, RainBranchResult{
					Branch:   b.Branch,
					Upstream: b.Upstream,
					Current:  b.Current,
					Outcome:  RainOutcomeFrozen,
					Message:  reason,
				})
				result.Frozen++
			}
			return result, nil
		}
	}

	checkedOut, err := checkedOutBranchesByWorktree(repoPath)
	if err != nil {
		return result, err
	}

	// Resolve effective branch sync mode.
	mode := opts.BranchMode
	if mode == "" {
		mode = BranchSyncMainline
	}

	// all-branches: create local tracking refs for remote branches not yet local.
	if mode == BranchSyncAllBranches && hasRemote {
		newBranches, trackErr := createLocalTrackingFromRemotes(repoPath, branches)
		if trackErr != nil {
			fmt.Fprintf(os.Stderr, "note: could not enumerate remote branches: %v\n", trackErr)
		} else {
			branches = append(branches, newBranches...)
		}
	}

	// Filter branches to those the selected mode covers.
	filtered := branches[:0]
	for _, b := range branches {
		switch mode {
		case BranchSyncMainline:
			if isMainlineBranch(b.Branch) || matchesPatterns(b.Branch, opts.MainlinePatterns) {
				filtered = append(filtered, b)
			}
		case BranchSyncCheckedOut:
			if checkedOut[b.Branch] {
				filtered = append(filtered, b)
			}
		default: // BranchSyncAllLocal, BranchSyncAllBranches
			filtered = append(filtered, b)
		}
	}
	branches = filtered

	hasStaged, err := HasStagedChanges(repoPath)
	if err != nil {
		return result, fmt.Errorf("check staged changes: %w", err)
	}
	hasUnstaged, err := HasUnstagedChanges(repoPath)
	if err != nil {
		return result, fmt.Errorf("check unstaged changes: %w", err)
	}
	currentDirty := hasStaged || hasUnstaged

	for _, branch := range branches {
		entry := RainBranchResult{
			Branch:   branch.Branch,
			Upstream: branch.Upstream,
			Current:  branch.Current,
		}

		record := func(outcome, message, backupBranch string) {
			entry.Outcome = outcome
			entry.Message = message
			entry.BackupBranch = backupBranch
			result.Branches = append(result.Branches, entry)
			switch outcome {
			case RainOutcomeUpdated, RainOutcomeUpdatedRisky:
				result.Updated++
			case RainOutcomeFrozen:
				result.Frozen++
			case RainOutcomeFailed:
				result.Failed++
			default:
				result.Skipped++
			}
		}

		if branch.Upstream == "" {
			inferred, inferErr := inferUpstreamRef(repoPath, branch.Branch)
			if inferErr != nil {
				record(RainOutcomeFailed, fmt.Sprintf("infer upstream: %v", inferErr), "")
				continue
			}
			if inferred == "" {
				record(RainOutcomeSkippedNoUpstream, "branch has no upstream", "")
				continue
			}
			branch.Upstream = inferred
			entry.Upstream = inferred
		}

		localSHA, localErr := getCommitSHA(repoPath, branch.Branch)
		if localErr != nil {
			record(RainOutcomeFailed, fmt.Sprintf("read local SHA: %v", localErr), "")
			continue
		}

		upstreamSHA, upstreamErr := getCommitSHA(repoPath, branch.Upstream)
		if upstreamErr != nil {
			record(RainOutcomeSkippedUpstreamMissing, "upstream ref missing after fetch", "")
			continue
		}

		localAhead, remoteAhead, divergeErr := branchDivergenceCounts(repoPath, branch.Branch, branch.Upstream)
		if divergeErr != nil {
			record(RainOutcomeFailed, fmt.Sprintf("compute divergence: %v", divergeErr), "")
			continue
		}

		if localAhead == 0 && remoteAhead == 0 {
			record(RainOutcomeUpToDate, "already up-to-date", "")
			continue
		}

		checkedOutElsewhere := !branch.Current && checkedOut[branch.Branch]

		if localAhead == 0 && remoteAhead > 0 {
			if branch.Current {
				mergeCmd := exec.Command("git", "merge", "--ff-only", branch.Upstream)
				mergeCmd.Dir = repoPath
				if output, mergeErr := mergeCmd.CombinedOutput(); mergeErr != nil {
					record(RainOutcomeSkippedUnsafeMerge, fmt.Sprintf("cannot fast-forward current branch safely: %s", strings.TrimSpace(string(output))), "")
					continue
				}
				record(RainOutcomeUpdated, "fast-forwarded current branch", "")
				continue
			}

			if checkedOutElsewhere {
				record(RainOutcomeSkippedCheckedOut, "branch is checked out in another worktree", "")
				continue
			}

			if updateErr := updateBranchRef(repoPath, branch.Branch, localSHA, upstreamSHA); updateErr != nil {
				record(RainOutcomeFailed, fmt.Sprintf("fast-forward ref update failed: %v", updateErr), "")
				continue
			}
			record(RainOutcomeUpdated, "fast-forwarded local ref", "")
			continue
		}

		if !opts.RiskyMode {
			if localAhead > 0 && remoteAhead == 0 {
				record(RainOutcomeSkippedLocalAhead, "local branch is ahead of upstream; risky mode disabled", "")
				continue
			}
			record(RainOutcomeSkippedDiverged, "branch diverged from upstream; risky mode disabled", "")
			continue
		}

		if branch.Current {
			if currentDirty {
				record(RainOutcomeSkippedUnsafeDirty, "current branch has staged or unstaged changes", "")
				continue
			}
			backupBranch, backupErr := createRainBackupBranch(repoPath, branch.Branch, localSHA)
			if backupErr != nil {
				record(RainOutcomeFailed, fmt.Sprintf("create backup branch failed: %v", backupErr), "")
				continue
			}
			resetCmd := exec.Command("git", "reset", "--hard", branch.Upstream)
			resetCmd.Dir = repoPath
			if output, resetErr := resetCmd.CombinedOutput(); resetErr != nil {
				record(RainOutcomeFailed, fmt.Sprintf("hard reset failed: %s", strings.TrimSpace(string(output))), backupBranch)
				continue
			}
			record(RainOutcomeUpdatedRisky, "realigned current branch to upstream via hard reset", backupBranch)
			continue
		}

		if checkedOutElsewhere {
			record(RainOutcomeSkippedCheckedOut, "branch is checked out in another worktree", "")
			continue
		}
		backupBranch, backupErr := createRainBackupBranch(repoPath, branch.Branch, localSHA)
		if backupErr != nil {
			record(RainOutcomeFailed, fmt.Sprintf("create backup branch failed: %v", backupErr), "")
			continue
		}
		if updateErr := updateBranchRef(repoPath, branch.Branch, localSHA, upstreamSHA); updateErr != nil {
			record(RainOutcomeFailed, fmt.Sprintf("destructive ref update failed: %v", updateErr), backupBranch)
			continue
		}
		record(RainOutcomeUpdatedRisky, "realigned local ref to upstream", backupBranch)
	}

	return result, nil
}

func repoHasAnyRemote(repoPath string) (bool, error) {
	cmd := exec.Command("git", "remote")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, commandError("git remote", err, out)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func inferUpstreamRef(repoPath, branch string) (string, error) {
	remotes, err := listRepoRemotes(repoPath)
	if err != nil {
		return "", err
	}
	candidates := make([]string, 0, len(remotes))
	for _, remote := range remotes {
		ref := fmt.Sprintf("%s/%s", remote, branch)
		if _, refErr := getCommitSHA(repoPath, ref); refErr == nil {
			candidates = append(candidates, ref)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return "", nil
	}
	return "", fmt.Errorf("multiple candidate upstream refs for %s: %s", branch, strings.Join(candidates, ", "))
}

func listRepoRemotes(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "remote")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, commandError("git remote", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	remotes := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			remotes = append(remotes, name)
		}
	}
	return remotes, nil
}

func listLocalBranchesWithUpstream(repoPath string) ([]localBranchTracking, error) {
	currentBranch, currentErr := GetCurrentBranch(repoPath)
	if currentErr != nil {
		currentBranch = ""
	}

	cmd := exec.Command("git", "for-each-ref", "refs/heads", "--format=%(refname:short)%09%(upstream:short)")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, commandError("git for-each-ref", err, out)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	branches := make([]localBranchTracking, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		branch := strings.TrimSpace(parts[0])
		upstream := ""
		if len(parts) == 2 {
			upstream = strings.TrimSpace(parts[1])
		}
		branches = append(branches, localBranchTracking{
			Branch:   branch,
			Upstream: upstream,
			Current:  branch != "" && branch == currentBranch,
		})
	}
	return branches, nil
}

func checkedOutBranchesByWorktree(repoPath string) (map[string]bool, error) {
	worktrees, err := ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch != "" {
			out[wt.Branch] = true
		}
	}
	return out, nil
}

func branchDivergenceCounts(repoPath, localRef, upstreamRef string) (int, int, error) {
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", localRef+"..."+upstreamRef)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, commandError("git rev-list --left-right --count", err, out)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", strings.TrimSpace(string(out)))
	}
	localAhead, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse local-ahead count: %w", err)
	}
	remoteAhead, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse remote-ahead count: %w", err)
	}
	return localAhead, remoteAhead, nil
}

func updateBranchRef(repoPath, branch, oldSHA, newSHA string) error {
	cmd := exec.Command("git", "update-ref", "refs/heads/"+branch, newSHA, oldSHA)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return commandError("git update-ref", err, out)
	}
	return nil
}

// createLocalTrackingFromRemotes enumerates remote-tracking refs and creates
// local tracking branches for any remote branch that does not yet exist locally.
// Returns the newly-created localBranchTracking entries.
func createLocalTrackingFromRemotes(repoPath string, existing []localBranchTracking) ([]localBranchTracking, error) {
	localSet := make(map[string]bool, len(existing))
	for _, b := range existing {
		localSet[b.Branch] = true
	}

	remotes, err := listRepoRemotes(repoPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "for-each-ref", "refs/remotes", "--format=%(refname:short)")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, commandError("git for-each-ref refs/remotes", err, out)
	}

	var created []localBranchTracking
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		var branchName, upstream string
		for _, remote := range remotes {
			prefix := remote + "/"
			if strings.HasPrefix(ref, prefix) {
				name := strings.TrimPrefix(ref, prefix)
				if name == "HEAD" {
					break
				}
				branchName = name
				upstream = ref
				break
			}
		}
		if branchName == "" || localSet[branchName] {
			continue
		}
		trackCmd := exec.Command("git", "branch", "--track", branchName, upstream)
		trackCmd.Dir = repoPath
		if trackOut, trackErr := trackCmd.CombinedOutput(); trackErr != nil {
			fmt.Fprintf(os.Stderr, "note: could not create local tracking branch %s: %v (%s)\n",
				branchName, trackErr, strings.TrimSpace(string(trackOut)))
			continue
		}
		localSet[branchName] = true
		created = append(created, localBranchTracking{
			Branch:   branchName,
			Upstream: upstream,
			Current:  false,
		})
	}
	return created, nil
}

func createRainBackupBranch(repoPath, branch, sha string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	branchSafe := strings.ReplaceAll(branch, "/", "-")
	backupBranch := fmt.Sprintf("git-rain-backup-%s-%s-%s", branchSafe, timestamp, shortSHA)
	cmd := exec.Command("git", "branch", backupBranch, sha)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", commandError("git branch "+backupBranch, err, out)
	}
	return backupBranch, nil
}
