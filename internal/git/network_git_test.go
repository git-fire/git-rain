package git

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	testutil "github.com/git-fire/git-testkit"
)

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

func TestRunNetworkGit_TimesOutStallRemote(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "stall-remote",
		Remotes: map[string]string{
			"origin": baseURL + "/stall.git",
		},
	})

	timeoutSec := 2
	start := time.Now()
	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	_, err := RunNetworkGit(context.Background(), cmd, NetworkGitParams{TimeoutSec: timeoutSec, MaxAttempts: 1})
	elapsed := time.Since(start)

	var timedOut *FetchTimedOutError
	if !errors.As(err, &timedOut) {
		t.Fatalf("RunNetworkGit() err = %v, want FetchTimedOutError", err)
	}
	if timedOut.Seconds != timeoutSec {
		t.Fatalf("timeout seconds = %d, want %d", timedOut.Seconds, timeoutSec)
	}
	if elapsed < time.Duration(timeoutSec)*time.Second {
		t.Fatalf("elapsed %v too short for %ds timeout", elapsed, timeoutSec)
	}
	if elapsed > time.Duration(timeoutSec+5)*time.Second {
		t.Fatalf("elapsed %v too long; fetch should not hang the test", elapsed)
	}
}

func TestRunNetworkGit_CancelStopsInFlightFetch(t *testing.T) {
	baseURL, cleanup := startStallTCPListener(t)
	defer cleanup()

	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "cancel-stall",
		Remotes: map[string]string{
			"origin": baseURL + "/stall.git",
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	_, err := RunNetworkGit(ctx, cmd, NetworkGitParams{TimeoutSec: 120, MaxAttempts: 1})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunNetworkGit() err = %v, want context.Canceled", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("cancel took %v; expected subprocess to stop quickly", elapsed)
	}
}

func TestRunNetworkGit_HealthyRemoteCompletes(t *testing.T) {
	scenario := testutil.NewScenario(t)
	remote := scenario.CreateBareRepo("upstream")
	local := scenario.CreateRepo("local").
		AddFile("README.md", "hi\n").
		Commit("init").
		WithRemote("origin", remote)
	local.Push("origin", local.GetDefaultBranch())

	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = local.Path()
	_, err := RunNetworkGit(context.Background(), cmd, NetworkGitParams{TimeoutSec: 30, MaxAttempts: 1})
	if err != nil {
		t.Fatalf("RunNetworkGit() on healthy remote: %v", err)
	}
}

func TestRunNetworkGit_TimeoutErrorNotDroppedByWaitRace(t *testing.T) {
	// CommandContext kill and the runCtx.Done select arm race; repeat so a
	// wrong waitDone path would flake as a generic "signal: killed" error.
	for i := 0; i < 40; i++ {
		cmd := exec.Command("sleep", "30")
		_, err := RunNetworkGit(context.Background(), cmd, NetworkGitParams{TimeoutSec: 1, MaxAttempts: 1})
		var timedOut *FetchTimedOutError
		if !errors.As(err, &timedOut) {
			t.Fatalf("iteration %d: err = %v (%T), want FetchTimedOutError", i, err, err)
		}
		if timedOut.Seconds != 1 {
			t.Fatalf("iteration %d: timeout seconds = %d, want 1", i, timedOut.Seconds)
		}
	}
}

func TestRunNetworkGit_ParentCancelKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group kill is not wired on Windows")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "child.pid")
	script := `#!/bin/sh
sleep 300 &
echo $! > "$1"
wait
`
	scriptPath := filepath.Join(dir, "stall.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("sh", scriptPath, marker)
	errCh := make(chan error, 1)
	go func() {
		_, err := RunNetworkGit(ctx, cmd, NetworkGitParams{TimeoutSec: 0, MaxAttempts: 1})
		errCh <- err
	}()

	var childPID int
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for child pid marker")
		}
		b, err := os.ReadFile(marker)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(b)))
			if convErr == nil && pid > 0 {
				childPID = pid
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunNetworkGit() err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunNetworkGit did not return after cancel")
	}

	// Child must not outlive the canceled process group.
	deadline = time.Now().Add(3 * time.Second)
	for {
		if err := exec.Command("kill", "-0", strconv.Itoa(childPID)).Run(); err != nil {
			return
		}
		if time.Now().After(deadline) {
			_ = exec.Command("kill", "-9", strconv.Itoa(childPID)).Run()
			t.Fatalf("grandchild pid %d still alive after parent cancel", childPID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
