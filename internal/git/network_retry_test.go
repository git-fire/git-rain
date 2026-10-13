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
		name string
		out  string
		err  error
		want bool
	}{
		{"timeout", "fatal: connection timed out", errors.New("exit 128"), true},
		{"auth", "terminal prompts disabled", errors.New("exit 128"), false},
		{"not found", "repository not found", errors.New("exit 128"), false},
		{"503", "error: RPC failed; HTTP 503 curl 22", errors.New("exit 128"), true},
		{
			"git url error 503",
			"fatal: unable to access 'https://example.com/repo.git': The requested URL returned error: 503\n",
			errors.New("exit 128"),
			true,
		},
		{
			"git url error 502",
			"fatal: unable to access 'https://example.com/repo.git': The requested URL returned error: 502\n",
			errors.New("exit 128"),
			true,
		},
		{
			"git url error 401",
			"fatal: unable to access 'https://example.com/repo.git': The requested URL returned error: 401\n",
			errors.New("exit 128"),
			false,
		},
		{
			"git 401 wins over transient phrase",
			"fatal: unable to access 'https://a/repo.git': The requested URL returned error: 401\nfatal: connection timed out\n",
			errors.New("exit 128"),
			false,
		},
		{"fetch timed out err", "", &FetchTimedOutError{Seconds: 2}, true},
		{"fetch timed out with auth", "fatal: Authentication failed for 'https://example.com/repo.git'", &FetchTimedOutError{Seconds: 2}, false},
		{"fetch timed out with prompts disabled", "fatal: could not read Username for 'https://example.com': terminal prompts disabled", &FetchTimedOutError{Seconds: 2}, false},
		{"fetch timed out with 403", "remote: HTTP 403\n fatal: unable to access", &FetchTimedOutError{Seconds: 2}, false},
		{
			"fetch timed out with repository url not found",
			"fatal: repository 'https://example.com/missing.git' not found\n",
			&FetchTimedOutError{Seconds: 2},
			false,
		},
		{
			"git url error 404 wins over timeout",
			"fatal: unable to access 'https://example.com/repo.git': The requested URL returned error: 404\n",
			&FetchTimedOutError{Seconds: 2},
			false,
		},
		{
			"pack object count 404 does not block timeout retry",
			"Total 404 (delta 12), reused 404 (delta 0), pack-reused 0\n",
			&FetchTimedOutError{Seconds: 2},
			true,
		},
		{
			"pack object count 404 does not block reset retry",
			"Total 404 (delta 12), reused 404 (delta 0), pack-reused 0\nfatal: connection reset by peer\n",
			errors.New("exit 128"),
			true,
		},
		{
			"invalid username or password wins over transient phrase",
			"fatal: Invalid username or password\nfatal: could not resolve host: other.example\n",
			errors.New("exit 128"),
			false,
		},
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
		MaxAttempts:       3,
		InitialBackoffSec: 1,
		MaxBackoffSec:     2,
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
	params := NetworkGitParams{MaxAttempts: 3, InitialBackoffSec: 1, MaxBackoffSec: 2}
	_, err := runNetworkGitWithRetry(context.Background(), cmd, params, stub)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on auth)", calls)
	}
}

func TestRunNetworkGitWithRetry_DoesNotRetryTimeoutAfterAuthOutput(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		return []byte("fatal: Authentication failed for 'https://example.com/a.git'\n"), &FetchTimedOutError{Seconds: 2}
	}
	cmd := exec.Command("git", "fetch", "--all")
	params := NetworkGitParams{MaxAttempts: 3, InitialBackoffSec: 1}
	_, err := runNetworkGitWithRetry(context.Background(), cmd, params, stub)
	if err == nil {
		t.Fatal("expected error")
	}
	var timedOut *FetchTimedOutError
	if !errors.As(err, &timedOut) {
		t.Fatalf("err = %v, want FetchTimedOutError", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry when timeout follows auth failure)", calls)
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
	params := NetworkGitParams{MaxAttempts: 5, InitialBackoffSec: 5, MaxBackoffSec: 5}
	_, err := runNetworkGitWithRetry(ctx, exec.Command("git", "fetch"), params, stub)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRunNetworkGitWithRetry_CapsInitialBackoffToMax(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		return []byte("connection timed out"), errors.New("exit")
	}
	start := time.Now()
	params := NetworkGitParams{MaxAttempts: 2, InitialBackoffSec: 60, MaxBackoffSec: 1}
	_, _ = runNetworkGitWithRetry(context.Background(), exec.Command("git", "fetch"), params, stub)
	elapsed := time.Since(start)
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("elapsed %v; max backoff should cap initial wait", elapsed)
	}
}

func TestRunNetworkGitWithRetry_ZeroBackoffImmediate(t *testing.T) {
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		if calls < 3 {
			return []byte("connection timed out"), errors.New("exit")
		}
		return []byte("ok"), nil
	}
	start := time.Now()
	params := NetworkGitParams{MaxAttempts: 3, InitialBackoffSec: 0, MaxBackoffSec: 0}
	_, err := runNetworkGitWithRetry(context.Background(), exec.Command("git", "fetch"), params, stub)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("runNetworkGitWithRetry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if elapsed > time.Second {
		t.Fatalf("elapsed %v; zero backoff should retry immediately", elapsed)
	}
}

func TestNetworkGitParamsFromRain_PreservesZero(t *testing.T) {
	p := NetworkGitParamsFromRain(RainOptions{
		FetchRetryMaxAttempts:       0,
		FetchRetryInitialBackoffSec: 0,
		FetchRetryMaxBackoffSec:     0,
	})
	if p.MaxAttempts != 0 || p.InitialBackoffSec != 0 || p.MaxBackoffSec != 0 {
		t.Fatalf("got %+v, want zeros preserved", p)
	}
	var calls int
	stub := func(context.Context, *exec.Cmd, int) ([]byte, error) {
		calls++
		return []byte("connection timed out"), errors.New("exit")
	}
	_, _ = runNetworkGitWithRetry(context.Background(), exec.Command("git", "fetch"), p, stub)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (zero max attempts => single try)", calls)
	}
}

func TestRunNetworkGit_RetriesTCPResetServer(t *testing.T) {
	var accepts atomic.Int32
	accepted := make(chan int, 8)
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
			n := int(accepts.Add(1))
			_ = c.Close()
			accepted <- n
		}
	}()

	stub := func(ctx context.Context, c *exec.Cmd, timeoutSec int) ([]byte, error) {
		conn, derr := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
		if derr != nil {
			return []byte(derr.Error()), derr
		}
		_ = conn.Close()
		select {
		case n := <-accepted:
			if n < 2 {
				return []byte("read: connection reset by peer"), errors.New("reset")
			}
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			return []byte("timed out waiting for accept"), errors.New("accept timeout")
		}
	}
	params := NetworkGitParams{MaxAttempts: 2, InitialBackoffSec: 1, MaxBackoffSec: 2}
	_, err = runNetworkGitWithRetry(context.Background(), exec.Command("git", "fetch"), params, stub)
	if err != nil {
		t.Fatalf("expected success on second attempt: %v", err)
	}
}
