package git

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type mainlineFetchTarget struct {
	local        string
	upstream     string
	current      bool
	remote       string
	remoteBranch string
}

// MainlineFetchRemotes runs a targeted git fetch per remote for branches whose
// upstream remote branch name qualifies as mainline (including user patterns).
// Only remote-tracking refs are updated; local branch refs are not moved.
func MainlineFetchRemotes(repoPath string, opts RainOptions) (RainResult, error) {
	result := RainResult{
		RepoPath: repoPath,
		Branches: make([]RainBranchResult, 0),
	}

	hasRemote, err := repoHasAnyRemote(repoPath)
	if err != nil {
		return result, err
	}
	if !hasRemote {
		return result, nil
	}

	branches, err := listLocalBranchesWithUpstream(repoPath)
	if err != nil {
		return result, err
	}
	if len(branches) == 0 {
		return result, nil
	}

	remotes, err := listRepoRemotes(repoPath)
	if err != nil {
		return result, err
	}
	if len(remotes) == 0 {
		return result, nil
	}

	var targets []mainlineFetchTarget
	for _, b := range branches {
		upstream := b.Upstream
		if upstream == "" {
			inferred, inferErr := inferUpstreamRef(repoPath, b.Branch)
			if inferErr != nil {
				if errors.Is(inferErr, ErrAmbiguousUpstream) {
					result.Branches = append(result.Branches, RainBranchResult{
						Branch:   b.Branch,
						Upstream: "",
						Current:  b.Current,
						Outcome:  RainOutcomeSkippedAmbiguousUpstream,
						Message:  fmt.Sprintf("infer upstream: %v", inferErr),
					})
					result.Skipped++
					continue
				}
				result.Branches = append(result.Branches, RainBranchResult{
					Branch:   b.Branch,
					Upstream: "",
					Current:  b.Current,
					Outcome:  RainOutcomeFailed,
					Message:  fmt.Sprintf("infer upstream: %v", inferErr),
				})
				result.Failed++
				continue
			}
			if inferred == "" {
				result.Branches = append(result.Branches, RainBranchResult{
					Branch:   b.Branch,
					Upstream: "",
					Current:  b.Current,
					Outcome:  RainOutcomeSkippedNoUpstream,
					Message:  "branch has no upstream",
				})
				result.Skipped++
				continue
			}
			upstream = inferred
		}

		remote, rb, ok := splitUpstreamRemoteBranch(upstream, remotes)
		if !ok || rb == "" || rb == "HEAD" {
			result.Branches = append(result.Branches, RainBranchResult{
				Branch:   b.Branch,
				Upstream: upstream,
				Current:  b.Current,
				Outcome:  RainOutcomeSkippedAmbiguousUpstream,
				Message:  "could not resolve remote ref for fetch",
			})
			result.Skipped++
			continue
		}

		if !isMainlineBranch(rb) && !matchesPatterns(rb, opts.MainlinePatterns) {
			continue
		}

		targets = append(targets, mainlineFetchTarget{
			local:        b.Branch,
			upstream:     upstream,
			current:      b.Current,
			remote:       remote,
			remoteBranch: rb,
		})
	}

	if len(targets) == 0 {
		return result, nil
	}

	byRemote := make(map[string]map[string]struct{})
	for _, t := range targets {
		if byRemote[t.remote] == nil {
			byRemote[t.remote] = make(map[string]struct{})
		}
		byRemote[t.remote][t.remoteBranch] = struct{}{}
	}

	failedReason := make(map[string]string)
	for remote, names := range byRemote {
		rbNames := make([]string, 0, len(names))
		for n := range names {
			rbNames = append(rbNames, n)
		}
		sort.Strings(rbNames)

		args := []string{"fetch", remote}
		if opts.FetchPrune {
			args = append(args, "--prune")
		}
		args = append(args, rbNames...)
		if opts.SyncTags {
			args = append(args, "--tags")
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			failedReason[remote] = fetchFailureReason(output)
		}
	}

	for _, t := range targets {
		entry := RainBranchResult{
			Branch:   t.local,
			Upstream: t.upstream,
			Current:  t.current,
		}
		if reason, failed := failedReason[t.remote]; failed {
			entry.Outcome = RainOutcomeFrozen
			entry.Message = reason
			result.Branches = append(result.Branches, entry)
			result.Frozen++
			continue
		}
		entry.Outcome = RainOutcomeFetched
		entry.Message = "fetched remote-tracking ref"
		result.Branches = append(result.Branches, entry)
		result.Updated++
	}

	return result, nil
}

func splitUpstreamRemoteBranch(upstream string, remotes []string) (remote, branch string, ok bool) {
	sorted := append([]string(nil), remotes...)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	for _, r := range sorted {
		prefix := r + "/"
		if strings.HasPrefix(upstream, prefix) {
			rest := strings.TrimPrefix(upstream, prefix)
			if rest != "" {
				return r, rest, true
			}
		}
	}
	return "", "", false
}
