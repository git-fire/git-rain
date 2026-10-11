package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/git-rain/git-rain/internal/git"
)

const jsonSchemaVersion = 1

// jsonBranchResult is one branch line in --json output (stable for agents).
type jsonBranchResult struct {
	Branch       string `json:"branch"`
	Upstream     string `json:"upstream,omitempty"`
	Outcome      string `json:"outcome"`
	Label        string `json:"label"`
	Message      string `json:"message,omitempty"`
	BackupBranch string `json:"backup_branch,omitempty"`
}

// jsonRepoResult is per-repository status in --json output.
type jsonRepoResult struct {
	Index          int                `json:"index"`
	Total          int                `json:"total,omitempty"`
	Name           string             `json:"name"`
	Path           string             `json:"path"`
	Operation      string             `json:"operation"`
	SummaryOutcome string             `json:"summary_outcome"`
	Error          string             `json:"error,omitempty"`
	Branches       []jsonBranchResult `json:"branches,omitempty"`
}

type jsonRunMeta struct {
	DryRun        bool   `json:"dry_run"`
	FullSync      bool   `json:"full_sync"`
	FetchMainline bool   `json:"fetch_mainline"`
	RiskyMode     bool   `json:"risky_mode"`
	BranchMode    string `json:"branch_mode"`
	FetchPrune    bool   `json:"fetch_prune"`
}

type jsonSummary struct {
	Repos   int `json:"repos"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Frozen  int `json:"frozen"`
	Failed  int `json:"failed"`
}

type jsonRunDocument struct {
	SchemaVersion int              `json:"schema_version"`
	Tool          string           `json:"tool"`
	Version       string           `json:"version"`
	FinishedAt    string           `json:"finished_at"`
	Run           jsonRunMeta      `json:"run"`
	Summary       jsonSummary      `json:"summary"`
	Repositories  []jsonRepoResult `json:"repositories"`
}

type jsonDryRunDocument struct {
	SchemaVersion int              `json:"schema_version"`
	Tool          string           `json:"tool"`
	Version       string           `json:"version"`
	FinishedAt    string           `json:"finished_at"`
	Run           jsonRunMeta      `json:"run"`
	Summary       jsonSummary      `json:"summary"`
	Repositories  []jsonDryRunRepo `json:"repositories"`
}

type jsonDryRunRepo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func jsonVersionString() string {
	v := Version
	if v == "" {
		return "dev"
	}
	return v
}

func buildJSONRunMeta(rainOpts git.RainOptions, fullSync bool) jsonRunMeta {
	return jsonRunMeta{
		DryRun:        rainDryRun,
		FullSync:      fullSync,
		FetchMainline: rainFetchMainline && !fullSync,
		RiskyMode:     rainOpts.RiskyMode,
		BranchMode:    string(rainOpts.BranchMode),
		FetchPrune:    rainOpts.FetchPrune,
	}
}

func branchesToJSON(branches []git.RainBranchResult, showBackup bool) []jsonBranchResult {
	if len(branches) == 0 {
		return nil
	}
	out := make([]jsonBranchResult, 0, len(branches))
	for _, br := range branches {
		entry := jsonBranchResult{
			Branch:  br.Branch,
			Outcome: br.Outcome,
			Label:   outcomeLabel(br.Outcome),
		}
		if br.Upstream != "" {
			entry.Upstream = br.Upstream
		}
		if br.Message != "" {
			entry.Message = br.Message
		}
		if showBackup && br.BackupBranch != "" {
			entry.BackupBranch = br.BackupBranch
		}
		out = append(out, entry)
	}
	return out
}

func writeJSONRun(w io.Writer, meta jsonRunMeta, totals rainTotals, repos []jsonRepoResult) error {
	sort.Slice(repos, func(i, j int) bool { return repos[i].Index < repos[j].Index })
	doc := jsonRunDocument{
		SchemaVersion: jsonSchemaVersion,
		Tool:          "git-rain",
		Version:       jsonVersionString(),
		FinishedAt:    time.Now().UTC().Format(time.RFC3339),
		Run:           meta,
		Summary: jsonSummary{
			Repos:   len(repos),
			Updated: totals.updated,
			Skipped: totals.skipped,
			Frozen:  totals.frozen,
			Failed:  totals.failed,
		},
		Repositories: repos,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func writeJSONDryRun(w io.Writer, meta jsonRunMeta, repos []git.Repository) error {
	list := make([]jsonDryRunRepo, 0, len(repos))
	for _, r := range repos {
		list = append(list, jsonDryRunRepo{Name: r.Name, Path: r.Path})
	}
	doc := jsonDryRunDocument{
		SchemaVersion: jsonSchemaVersion,
		Tool:          "git-rain",
		Version:       jsonVersionString(),
		FinishedAt:    time.Now().UTC().Format(time.RFC3339),
		Run:           meta,
		Summary:       jsonSummary{Repos: len(list)},
		Repositories:  list,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func emitJSONRun(meta jsonRunMeta, totals rainTotals, repos []jsonRepoResult) error {
	if err := writeJSONRun(os.Stdout, meta, totals, repos); err != nil {
		return fmt.Errorf("write json output: %w", err)
	}
	return nil
}

func emitJSONDryRun(meta jsonRunMeta, repos []git.Repository) error {
	if err := writeJSONDryRun(os.Stdout, meta, repos); err != nil {
		return fmt.Errorf("write json output: %w", err)
	}
	return nil
}
