package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"

	"github.com/git-rain/git-rain/internal/git"
)

func TestRunRain_JSONDefaultFetch(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := makeFetchedRepo(t, scenario, "json-fetch")

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainJSON = true

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := runRain(rootCmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("runRain json error = %v\n%s", runErr, buf.String())
	}

	var doc jsonRunDocument
	if err := json.Unmarshal([]byte(buf.String()), &doc); err != nil {
		t.Fatalf("json decode: %v\nraw:\n%s", err, buf.String())
	}
	if doc.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("schema_version = %d", doc.SchemaVersion)
	}
	if doc.Tool != "git-rain" {
		t.Fatalf("tool = %q", doc.Tool)
	}
	if len(doc.Repositories) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(doc.Repositories))
	}
	if doc.Repositories[0].Operation != "fetch-all" {
		t.Fatalf("operation = %q", doc.Repositories[0].Operation)
	}
	if doc.Repositories[0].SummaryOutcome != "fetched" {
		t.Fatalf("summary_outcome = %q", doc.Repositories[0].SummaryOutcome)
	}
	if doc.Summary.Updated != 1 {
		t.Fatalf("summary.updated = %d", doc.Summary.Updated)
	}
}

func TestRunRain_DryRunJSON(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	repo := makeFetchedRepo(t, scenario, "json-dry")

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainDryRun = true
	rainJSON = true

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := runRain(rootCmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("runRain dry-run json error = %v\n%s", runErr, buf.String())
	}

	var doc jsonDryRunDocument
	if err := json.Unmarshal([]byte(buf.String()), &doc); err != nil {
		t.Fatalf("json decode: %v\nraw:\n%s", err, buf.String())
	}
	if !doc.Run.DryRun {
		t.Fatal("expected run.dry_run true")
	}
	if len(doc.Repositories) != 1 {
		t.Fatalf("expected 1 repo in dry run json, got %d", len(doc.Repositories))
	}
}

func TestRunRain_NonInteractiveRejectsRainFlag(t *testing.T) {
	resetFlags()
	rainRain = true
	rainNonInteractive = true

	if err := runRain(rootCmd, nil); err == nil {
		t.Fatal("expected error for --rain with --non-interactive")
	}
}

func TestRunRain_JSONSkippedSummaryOutcome(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("remote-json-skip")
	repo := scenario.CreateRepo("json-skip-ahead").
		WithRemote("origin", remote).
		AddFile("tracked.txt", "v1\n").
		Commit("init")
	repo.Push("origin", repo.GetDefaultBranch())
	repo.AddFile("local-only.txt", "ahead\n").Commit("local ahead")

	resetFlags()
	rainPath = filepath.Dir(repo.Path())
	rainSync = true
	rainJSON = true

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := runRain(rootCmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("runRain json sync error = %v\n%s", runErr, buf.String())
	}

	var doc jsonRunDocument
	if err := json.Unmarshal([]byte(buf.String()), &doc); err != nil {
		t.Fatalf("json decode: %v\nraw:\n%s", err, buf.String())
	}
	if len(doc.Repositories) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(doc.Repositories))
	}
	if doc.Repositories[0].SummaryOutcome != "skipped" {
		t.Fatalf("summary_outcome = %q, want skipped (local-ahead under safe --sync)", doc.Repositories[0].SummaryOutcome)
	}
	if doc.Summary.Skipped < 1 {
		t.Fatalf("summary.skipped = %d, want >= 1", doc.Summary.Skipped)
	}
}

func TestWriteJSONRun_NilReposEncodesEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSONRun(&buf, jsonRunMeta{}, rainTotals{}, nil); err != nil {
		t.Fatalf("writeJSONRun: %v", err)
	}
	raw := buf.String()
	if strings.Contains(raw, `"repositories": null`) {
		t.Fatalf("nil repos must encode as [], got:\n%s", raw)
	}
	var doc jsonRunDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("json decode: %v\nraw:\n%s", err, raw)
	}
	if doc.Repositories == nil {
		t.Fatal("decoded repositories must be non-nil empty slice")
	}
	if len(doc.Repositories) != 0 {
		t.Fatalf("repositories len = %d, want 0", len(doc.Repositories))
	}
}

func TestBuildJSONRunMeta_FetchPruneReflectsCLI(t *testing.T) {
	tests := []struct {
		name    string
		prune   bool
		noPrune bool
		global  bool
		want    bool
	}{
		{"global off, no CLI", false, false, false, false},
		{"global on, no CLI", false, false, true, true},
		{"--prune overrides global off", true, false, false, true},
		{"--no-prune overrides global on", false, true, true, false},
		{"--no-prune wins over --prune", true, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			rainPrune = tt.prune
			rainNoPrune = tt.noPrune
			meta := buildJSONRunMeta(git.RainOptions{FetchPrune: tt.global, BranchMode: git.BranchSyncMainline}, false)
			if meta.FetchPrune != tt.want {
				t.Fatalf("FetchPrune = %v, want %v", meta.FetchPrune, tt.want)
			}
		})
	}
}

func TestRunRain_JSONEmptyScanUsesReposArray(t *testing.T) {
	tmpHome := t.TempDir()
	setTestUserDirs(t, tmpHome)

	resetFlags()
	rainPath = t.TempDir()
	rainJSON = true

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := runRain(rootCmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("runRain json empty error = %v\n%s", runErr, buf.String())
	}
	raw := buf.String()
	if strings.Contains(raw, `"repositories": null`) {
		t.Fatalf("empty live JSON must not use null repositories:\n%s", raw)
	}
	var doc jsonRunDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json decode: %v\nraw:\n%s", err, raw)
	}
	if doc.Repositories == nil {
		t.Fatal("repositories decoded as nil")
	}
	if doc.Summary.Repos != 0 {
		t.Fatalf("summary.repos = %d", doc.Summary.Repos)
	}
}
