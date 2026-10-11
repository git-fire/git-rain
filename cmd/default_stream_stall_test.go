package cmd

import (
	"bytes"
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
