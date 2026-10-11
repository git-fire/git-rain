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
	TimeoutSec          int
	MaxAttempts         int
	InitialBackoffSec   int
	MaxBackoffSec       int
}

// NetworkGitParamsFromRain maps RainOptions into network git execution params.
func NetworkGitParamsFromRain(opts RainOptions) NetworkGitParams {
	maxAttempts := opts.FetchRetryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultFetchRetryMaxAttempts
	}
	return NetworkGitParams{
		TimeoutSec:        opts.FetchTimeoutSec,
		MaxAttempts:       maxAttempts,
		InitialBackoffSec: opts.FetchRetryInitialBackoffSec,
		MaxBackoffSec:     opts.FetchRetryMaxBackoffSec,
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
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	initialBackoff := params.InitialBackoffSec
	if initialBackoff <= 0 {
		initialBackoff = DefaultFetchRetryInitialBackoffSec
	}
	maxBackoff := params.MaxBackoffSec
	if maxBackoff <= 0 {
		maxBackoff = DefaultFetchRetryMaxBackoffSec
	}

	var lastOut []byte
	var lastErr error
	backoff := time.Duration(initialBackoff) * time.Second

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
		if !sleepWithContext(parent, backoff) {
			return lastOut, parent.Err()
		}
		if backoff < time.Duration(maxBackoff)*time.Second {
			backoff *= 2
			if backoff > time.Duration(maxBackoff)*time.Second {
				backoff = time.Duration(maxBackoff) * time.Second
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
	var timedOut *FetchTimedOutError
	if errors.As(err, &timedOut) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	s := strings.ToLower(string(output) + " " + err.Error())

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
	}
	for _, p := range nonRetry {
		if strings.Contains(s, p) {
			return false
		}
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
