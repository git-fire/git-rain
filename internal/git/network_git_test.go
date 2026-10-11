package git

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
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
	_, err := RunNetworkGit(context.Background(), cmd, timeoutSec)
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
	_, err := RunNetworkGit(ctx, cmd, 120)
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
	_, err := RunNetworkGit(context.Background(), cmd, 30)
	if err != nil {
		t.Fatalf("RunNetworkGit() on healthy remote: %v", err)
	}
}
