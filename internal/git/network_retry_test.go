package git

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTransientNetworkFailure_Classification(t *testing.T) {
	cases := []struct {
		name   string
		out    string
		err    error
		want   bool
	}{
		{"timeout", "fatal: connection timed out", errors.New("exit 128"), true},
		{"auth", "terminal prompts disabled", errors.New("exit 128"), false},
		{"not found", "repository not found", errors.New("exit 128"), false},
		{"503", "error: RPC failed; HTTP 503 curl 22", errors.New("exit 128"), true},
		{"fetch timed out err", "", &FetchTimedOutError{Seconds: 2}, true},
	}
	for _, tc := range cases {
		if got := isTransientNetworkFailure([]byte(tc.out), tc.err); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestRunNetworkGitWithRetry_RetriesTransientThenSucceeds(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		if calls < 3 {
			return []byte("fatal: connection timed out\n"), errors.New("exit 128")
		}
		return []byte("ok"), nil
	}
	cmd := exec.Command("git", "fetch")
	params := NetworkGitParams{
		MaxAttempts:         3,
		InitialBackoffSec:   1,
		MaxBackoffSec:       2,
	}
	out, err := runNetworkGitWithRetry(context.Background(), cmd, params, stub)
	if err != nil {
		t.Fatalf("runNetworkGitWithRetry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if string(out) != "ok" {
		t.Fatalf("out = %q", out)
	}
}

func TestRunNetworkGitWithRetry_DoesNotRetryAuth(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		return []byte("terminal prompts disabled"), errors.New("exit 128")
	}
	cmd := exec.Command("git", "fetch")
	params := NetworkGitParams{MaxAttempts: 3, InitialBackoffSec: 1}
	_, err := runNetworkGitWithRetry(context.Background(), cmd, params, stub)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on auth)", calls)
	}
}

func TestRunNetworkGit_RetriesFlakyHTTPServer(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()

	repo := t.TempDir()
	cmd := exec.Command("git", "fetch", srv.URL)
	cmd.Dir = repo

	params := NetworkGitParams{
		MaxAttempts:       3,
		InitialBackoffSec: 1,
		MaxBackoffSec:     2,
		TimeoutSec:        5,
	}

	stub := func(ctx context.Context, c *exec.Cmd, timeoutSec int) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return []byte(err.Error()), err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusServiceUnavailable {
			return []byte("error: RPC failed; HTTP 503"), errors.New("exit 128")
		}
		return []byte("ok"), nil
	}

	_, err := runNetworkGitWithRetry(context.Background(), cmd, params, stub)
	if err != nil {
		t.Fatalf("expected success after retries, got %v (hits=%d)", err, hits.Load())
	}
	if hits.Load() != 3 {
		t.Fatalf("server hits = %d, want 3", hits.Load())
	}
}

func TestRunNetworkGitWithRetry_RespectsCancellationDuringBackoff(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		return []byte("connection timed out"), errors.New("exit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	params := NetworkGitParams{MaxAttempts: 5, InitialBackoffSec: 5}
	_, err := runNetworkGitWithRetry(ctx, exec.Command("git", "fetch"), params, stub)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRunNetworkGit_RetriesTCPResetServer(t *testing.T) {
	var accepts atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			if accepts.Add(1) < 2 {
				_ = c.Close()
				continue
			}
			_ = c.Close()
		}
	}()

	stub := func(ctx context.Context, c *exec.Cmd, timeoutSec int) ([]byte, error) {
		conn, derr := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
		if derr != nil {
			return []byte(derr.Error()), derr
		}
		_ = conn.Close()
		if accepts.Load() < 2 {
			return []byte("read: connection reset by peer"), errors.New("reset")
		}
		return nil, nil
	}
	params := NetworkGitParams{MaxAttempts: 2, InitialBackoffSec: 1}
	_, err = runNetworkGitWithRetry(context.Background(), exec.Command("git", "fetch"), params, stub)
	if err != nil {
		t.Fatalf("expected success on second attempt: %v", err)
	}
}
