package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultFetchRetryMaxAttempts       = 2
	DefaultFetchRetryInitialBackoffSec = 2
	DefaultFetchRetryMaxBackoffSec     = 30
)

// NetworkGitParams configures timeout and retry behavior for RunNetworkGit.
type NetworkGitParams struct {
	TimeoutSec        int
	MaxAttempts       int
	InitialBackoffSec int
	MaxBackoffSec     int
	OnRetry           func(attempt int, maxAttempts int)
}

// NetworkGitParamsFromRain maps RainOptions into network git execution params.
// Zero values are preserved so callers can disable retries (0 attempts) or use
// immediate backoff (0 seconds). Negative values mean "unset".
func NetworkGitParamsFromRain(opts RainOptions) NetworkGitParams {
	return NetworkGitParams{
		TimeoutSec:        opts.FetchTimeoutSec,
		MaxAttempts:       opts.FetchRetryMaxAttempts,
		InitialBackoffSec: opts.FetchRetryInitialBackoffSec,
		MaxBackoffSec:     opts.FetchRetryMaxBackoffSec,
		OnRetry:           opts.OnFetchAttempt,
	}
}

type networkGitAttemptFunc func(context.Context, *exec.Cmd, int) ([]byte, error)

// RunNetworkGit runs a git command that may contact a remote, with optional
// retries for transient network failures.
func RunNetworkGit(parent context.Context, cmd *exec.Cmd, params NetworkGitParams) ([]byte, error) {
	return runNetworkGitWithRetry(parent, cmd, params, runNetworkGitOnce)
}

func runNetworkGitWithRetry(parent context.Context, cmd *exec.Cmd, params NetworkGitParams, attempt networkGitAttemptFunc) ([]byte, error) {
	if parent == nil {
		parent = context.Background()
	}
	maxAttempts := params.MaxAttempts
	if maxAttempts < 0 {
		maxAttempts = DefaultFetchRetryMaxAttempts
	} else if maxAttempts == 0 {
		maxAttempts = 1
	}
	initialBackoff := params.InitialBackoffSec
	if initialBackoff < 0 {
		initialBackoff = DefaultFetchRetryInitialBackoffSec
	}
	maxBackoff := params.MaxBackoffSec
	if maxBackoff < 0 {
		maxBackoff = DefaultFetchRetryMaxBackoffSec
	}

	var lastOut []byte
	var lastErr error
	maxBackoffDur := time.Duration(maxBackoff) * time.Second
	backoff := time.Duration(initialBackoff) * time.Second
	if backoff > maxBackoffDur {
		backoff = maxBackoffDur
	}

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		if parent.Err() != nil {
			return lastOut, parent.Err()
		}
		out, err := attempt(parent, cmd, params.TimeoutSec)
		lastOut, lastErr = out, err
		if err == nil {
			return out, nil
		}
		if attemptNum >= maxAttempts || !isTransientNetworkFailure(out, err) {
			return out, err
		}
		if params.OnRetry != nil {
			params.OnRetry(attemptNum+1, maxAttempts)
		}
		if !sleepWithContext(parent, backoff) {
			return lastOut, parent.Err()
		}
		if backoff < maxBackoffDur {
			backoff *= 2
			if backoff > maxBackoffDur {
				backoff = maxBackoffDur
			}
		}
	}
	return lastOut, lastErr
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func isTransientNetworkFailure(output []byte, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	s := strings.ToLower(string(output) + " " + err.Error())

	// Auth / permanent failures must win over FetchTimedOutError: a multi-remote
	// fetch can log authentication failed (or similar) and still hit the attempt
	// deadline if another remote hangs.
	nonRetry := []string{
		"authentication failed",
		"could not read username",
		"could not read password",
		"permission denied",
		"repository not found",
		"terminal prompts disabled",
		"invalid credentials",
		" 401 ",
		" 403 ",
		"http 401",
		"http 403",
		// git: "The requested URL returned error: 401\n"
		"error: 401",
		"error: 403",
	}
	for _, p := range nonRetry {
		if strings.Contains(s, p) {
			return false
		}
	}

	var timedOut *FetchTimedOutError
	if errors.As(err, &timedOut) {
		return true
	}

	transient := []string{
		"could not resolve host",
		"connection timed out",
		"connection refused",
		"reset by peer",
		"broken pipe",
		"empty reply from server",
		"failed to connect",
		"http 502",
		"http 503",
		"http 504",
		" 502 ",
		" 503 ",
		" 504 ",
		// git: "The requested URL returned error: 503\n"
		"error: 502",
		"error: 503",
		"error: 504",
		"timed out after",
		"operation timed out",
		"network is unreachable",
		"no route to host",
		"temporary failure in name resolution",
	}
	for _, p := range transient {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
