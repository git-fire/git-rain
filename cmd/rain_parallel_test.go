package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	testutil "github.com/git-fire/git-testkit"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/rainexec"
	"github.com/git-rain/git-rain/internal/registry"
)

func TestRunRainOnRepos_ParallelStallDoesNotBlockHealthy(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("upstream")
	healthy := scenario.CreateRepo("healthy").
		AddFile("README.md", "ok\n").
		Commit("init").
		WithRemote("origin", remote)
	healthy.Push("origin", healthy.GetDefaultBranch())

	stall := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "stall",
		Remotes: map[string]string{
			"origin": baseURL + "/stall.git",
		},
	})

	repos := []git.Repository{
		{Name: "healthy", Path: healthy.Path()},
		{Name: "stall", Path: stall},
	}

	timeoutSec := 2
	rainOpts := git.RainOptions{FetchTimeoutSec: timeoutSec}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	start := time.Now()
	err := runRainOnRepos(&registry.Registry{}, repos, rainOpts, false, 2) // CLI path (parallel stdout)
	elapsed := time.Since(start)

	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)

	if err != nil {
		t.Fatalf("runRainOnRepos: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "rain delivered") {
		t.Fatalf("expected summary, got:\n%s", body)
	}
	if !strings.Contains(body, "timed out") {
		t.Fatalf("expected stall timeout in output, got:\n%s", body)
	}
	if elapsed > time.Duration(timeoutSec+8)*time.Second {
		t.Fatalf("parallel rain took %v; expected ~%ds timeout bound", elapsed, timeoutSec)
	}
}

func TestRunRainReposParallel_PreservesSelectionIndex(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("upstream")
	fast := scenario.CreateRepo("repo-alpha").
		AddFile("README.md", "ok\n").
		Commit("init").
		WithRemote("origin", remote)
	fast.Push("origin", fast.GetDefaultBranch())

	stall := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "repo-beta",
		Remotes: map[string]string{
			"origin": baseURL + "/stall.git",
		},
	})

	repos := []git.Repository{
		{Name: "repo-alpha", Path: fast.Path()},
		{Name: "repo-beta", Path: stall},
	}

	oldJSON := rainJSON
	rainJSON = true
	defer func() { rainJSON = oldJSON }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	totals, reports := runRainReposParallel(&registry.Registry{}, repos, git.RainOptions{FetchTimeoutSec: 2}, false, 2, nil, nil, false)
	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)

	if totals.updated < 1 {
		t.Fatalf("expected at least one updated repo, totals=%+v", totals)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 json reports, got %d", len(reports))
	}

	byName := map[string]jsonRepoResult{}
	for _, rep := range reports {
		byName[rep.Name] = rep
	}
	if byName["repo-alpha"].Index != 1 || byName["repo-alpha"].Total != 2 {
		t.Fatalf("repo-alpha index/total = %d/%d, want 1/2", byName["repo-alpha"].Index, byName["repo-alpha"].Total)
	}
	if byName["repo-beta"].Index != 2 || byName["repo-beta"].Total != 2 {
		t.Fatalf("repo-beta index/total = %d/%d, want 2/2", byName["repo-beta"].Index, byName["repo-beta"].Total)
	}

	var doc jsonRunDocument
	if err := json.Unmarshal([]byte(out.String()), &doc); err == nil && len(doc.Repositories) > 0 {
		for _, rep := range doc.Repositories {
			want := byName[rep.Name].Index
			if rep.Index != want {
				t.Fatalf("stdout json repo %s index %d, want %d", rep.Name, rep.Index, want)
			}
		}
	}
}

func TestRunRainReposParallel_PreStartCancelCountsSkipped(t *testing.T) {
	repos := []git.Repository{{Name: "pending", Path: "/tmp/git-rain-never-started"}}
	coord := newRainCoordinator(context.Background(), []string{repos[0].Path})
	coord.SkipRepo(repos[0].Path)

	progress := make(chan rainexec.Event, 4)
	totals, _ := runRainReposParallel(&registry.Registry{}, repos, git.RainOptions{}, false, 1, progress, coord, true)
	close(progress)

	if totals.skipped != 1 {
		t.Fatalf("totals.skipped = %d, want 1 (pre-start cancel must count in summary)", totals.skipped)
	}
	if totals.updated != 0 || totals.frozen != 0 || totals.failed != 0 {
		t.Fatalf("unexpected non-skip totals: %+v", totals)
	}

	var sawSkipped bool
	for e := range progress {
		if e.State == rainexec.StateSkipped {
			sawSkipped = true
		}
	}
	if !sawSkipped {
		t.Fatal("expected skipped progress event for pre-start cancel")
	}
}
