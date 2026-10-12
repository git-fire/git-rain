package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/git-rain/git-rain/internal/config"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
	testutil "github.com/git-fire/git-testkit"
)

func TestRunRainDefaultStream_StallRemoteDoesNotBlockSummary(t *testing.T) {
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

	timeoutSec := 2
	cfg := config.DefaultConfig()
	cfg.Global.FetchWorkers = 2
	cfg.Global.FetchTimeoutSec = timeoutSec
	cfg.Global.DisableScan = true

	opts := git.DefaultScanOptions()
	opts.DisableScan = true
	opts.KnownPaths = map[string]bool{
		healthy.Path(): true,
		stall:          true,
	}
	opts.Workers = 2

	rainOpts := git.RainOptions{FetchTimeoutSec: timeoutSec}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	start := time.Now()
	err := runRainDefaultStream(&cfg, &registry.Registry{}, "", opts, rainOpts, false)
	elapsed := time.Since(start)

	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)

	if err != nil {
		t.Fatalf("runRainDefaultStream: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "rain delivered") {
		t.Fatalf("expected summary line in output, got:\n%s", body)
	}
	if !strings.Contains(body, "timed out") {
		t.Fatalf("expected stall repo to report timeout in output, got:\n%s", body)
	}
	if elapsed > time.Duration(timeoutSec+8)*time.Second {
		t.Fatalf("run took %v; should finish soon after stall timeout %ds", elapsed, timeoutSec)
	}
}

func TestRunRainDefaultStream_CancelStopsRemainingRepos(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	stallA := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "stall-a",
		Remotes: map[string]string{
			"origin": baseURL + "/stall-a.git",
		},
	})
	stallB := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "stall-b",
		Remotes: map[string]string{
			"origin": baseURL + "/stall-b.git",
		},
	})
	stallC := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "stall-c",
		Remotes: map[string]string{
			"origin": baseURL + "/stall-c.git",
		},
	})

	cfg := config.DefaultConfig()
	cfg.Global.FetchWorkers = 1
	cfg.Global.FetchTimeoutSec = 30
	cfg.Global.DisableScan = true

	opts := git.DefaultScanOptions()
	opts.DisableScan = true
	opts.KnownPaths = map[string]bool{
		stallA: true,
		stallB: true,
		stallC: true,
	}
	opts.Workers = 1

	runCtx, cancel := context.WithCancel(context.Background())
	rainOpts := git.RainOptions{
		FetchTimeoutSec: 30,
		RunCtx:          runCtx,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		done <- runRainDefaultStream(&cfg, &registry.Registry{}, "", opts, rainOpts, false)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRainDefaultStream: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runRainDefaultStream did not return after cancel")
	}
	elapsed := time.Since(start)

	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)
	body := out.String()

	if !strings.Contains(body, "Aborted.") {
		t.Fatalf("expected Aborted. after cancel, got:\n%s", body)
	}
	if strings.Contains(body, "rain delivered") {
		t.Fatalf("did not expect completed summary after cancel, got:\n%s", body)
	}
	// With one worker and three stalled remotes, cancel must not fetch all three
	// to completion (each would otherwise hang until the 30s timeout).
	if elapsed > 5*time.Second {
		t.Fatalf("run took %v after cancel; remaining repos should be skipped", elapsed)
	}
	frozenLines := strings.Count(body, "❄")
	if frozenLines >= 3 {
		t.Fatalf("expected remaining repos skipped after cancel, got %d frozen lines:\n%s", frozenLines, body)
	}
}

func TestRunRainDefaultStream_JSONCancelEmitsJSONNotAborted(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	stallA := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "json-stall-a",
		Remotes: map[string]string{
			"origin": baseURL + "/json-stall-a.git",
		},
	})
	stallB := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "json-stall-b",
		Remotes: map[string]string{
			"origin": baseURL + "/json-stall-b.git",
		},
	})

	prevJSON := rainJSON
	rainJSON = true
	defer func() { rainJSON = prevJSON }()

	cfg := config.DefaultConfig()
	cfg.Global.FetchWorkers = 1
	cfg.Global.FetchTimeoutSec = 30
	cfg.Global.DisableScan = true

	opts := git.DefaultScanOptions()
	opts.DisableScan = true
	opts.KnownPaths = map[string]bool{
		stallA: true,
		stallB: true,
	}
	opts.Workers = 1

	runCtx, cancel := context.WithCancel(context.Background())
	rainOpts := git.RainOptions{
		FetchTimeoutSec: 30,
		RunCtx:          runCtx,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	done := make(chan error, 1)
	go func() {
		done <- runRainDefaultStream(&cfg, &registry.Registry{}, "", opts, rainOpts, false)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected non-nil error on JSON cancel")
		}
		if !strings.Contains(err.Error(), "interrupted") {
			t.Fatalf("expected interrupted error, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runRainDefaultStream did not return after cancel")
	}

	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)
	body := out.String()

	if strings.Contains(body, "Aborted.") {
		t.Fatalf("--json cancel must not print Aborted. on stdout, got:\n%s", body)
	}
	var doc jsonRunDocument
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("expected schema_version 1 JSON on cancel, decode error: %v\nraw:\n%s", err, body)
	}
	if doc.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("schema_version = %d", doc.SchemaVersion)
	}
}

func TestRunRainOnRepos_CancelSkipsRemaining(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	stallA := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "seq-stall-a",
		Remotes: map[string]string{
			"origin": baseURL + "/seq-stall-a.git",
		},
	})
	stallB := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "seq-stall-b",
		Remotes: map[string]string{
			"origin": baseURL + "/seq-stall-b.git",
		},
	})
	stallC := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "seq-stall-c",
		Remotes: map[string]string{
			"origin": baseURL + "/seq-stall-c.git",
		},
	})

	runCtx, cancel := context.WithCancel(context.Background())
	rainOpts := git.RainOptions{
		FetchTimeoutSec: 30,
		RunCtx:          runCtx,
	}
	repos := []git.Repository{
		{Name: "seq-stall-a", Path: stallA, Selected: true},
		{Name: "seq-stall-b", Path: stallB, Selected: true},
		{Name: "seq-stall-c", Path: stallC, Selected: true},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		done <- runRainOnRepos(&registry.Registry{}, repos, rainOpts, false, 1)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRainOnRepos: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runRainOnRepos did not return after cancel")
	}
	elapsed := time.Since(start)

	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)
	body := out.String()

	if !strings.Contains(body, "Aborted.") {
		t.Fatalf("expected Aborted. after cancel, got:\n%s", body)
	}
	if strings.Contains(body, "rain delivered") {
		t.Fatalf("did not expect completed summary after cancel, got:\n%s", body)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("run took %v after cancel; remaining repos should be skipped", elapsed)
	}
	frozenLines := strings.Count(body, "❄")
	if frozenLines >= 3 {
		t.Fatalf("expected remaining repos skipped after cancel, got %d frozen lines:\n%s", frozenLines, body)
	}
}

func TestRunDryRun_CancelAborts(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before scan

	cfg := config.DefaultConfig()
	opts := git.DefaultScanOptions()
	opts.RootPath = t.TempDir()
	opts.DisableScan = false
	opts.Workers = 1

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err := runDryRun(opts, git.RainOptions{RunCtx: runCtx}, false, &cfg)
	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)
	body := out.String()

	if err != nil {
		t.Fatalf("human dry-run cancel should return nil, got %v", err)
	}
	if !strings.Contains(body, "Aborted.") {
		t.Fatalf("expected Aborted. after dry-run cancel, got:\n%s", body)
	}
	if strings.Contains(body, "Would run") {
		t.Fatalf("did not expect dry-run listing after cancel, got:\n%s", body)
	}
}

func TestRunDryRun_JSONCancelReturnsError(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	cancel()

	prevJSON := rainJSON
	rainJSON = true
	defer func() { rainJSON = prevJSON }()

	cfg := config.DefaultConfig()
	opts := git.DefaultScanOptions()
	opts.RootPath = t.TempDir()
	opts.Workers = 1

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err := runDryRun(opts, git.RainOptions{RunCtx: runCtx}, false, &cfg)
	_ = w.Close()
	var out bytes.Buffer
	_, _ = io.Copy(&out, r)
	body := out.String()

	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("expected interrupted error, got %v", err)
	}
	if strings.Contains(body, "Aborted.") {
		t.Fatalf("--json dry-run cancel must not print Aborted., got:\n%s", body)
	}
}

func startStallTCPListener(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				select {
				case <-done:
					_ = c.Close()
				case <-time.After(10 * time.Minute):
					_ = c.Close()
				}
			}(conn)
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	return fmt.Sprintf("http://%s:%s", host, port), func() {
		close(done)
		_ = ln.Close()
	}
}
