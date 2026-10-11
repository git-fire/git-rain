package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
	testutil "github.com/git-fire/git-testkit"
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
	err := runRainOnRepos(&registry.Registry{}, repos, rainOpts, false, 2)
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
