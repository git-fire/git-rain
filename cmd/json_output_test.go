package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"
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
