package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrFetchTimedOut is returned when a network git subprocess exceeds FetchTimeoutSec.
var ErrFetchTimedOut = errors.New("fetch timed out")

// FetchTimedOutError carries the configured timeout for user-facing messages.
type FetchTimedOutError struct {
	Seconds int
}

func (e *FetchTimedOutError) Error() string {
	return fmt.Sprintf("timed out after %ds", e.Seconds)
}

func (e *FetchTimedOutError) Is(target error) bool {
	return target == ErrFetchTimedOut
}

// runNetworkGitOnce executes a single network git attempt (no retries).
func runNetworkGitOnce(parent context.Context, cmd *exec.Cmd, timeoutSec int) ([]byte, error) {
	if parent == nil {
		parent = context.Background()
	}
	runCtx := parent
	var cancel context.CancelFunc
	if timeoutSec > 0 {
		runCtx, cancel = context.WithTimeout(parent, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	if cmd.Path == "" {
		cmd.Path = cmd.Args[0]
	}
	wrapped := exec.CommandContext(runCtx, cmd.Path, cmd.Args[1:]...)
	wrapped.Dir = cmd.Dir
	if cmd.Env != nil {
		wrapped.Env = cmd.Env
	}
	PrepareNetworkGit(wrapped)
	setCmdProcessGroup(wrapped)

	var combined bytes.Buffer
	wrapped.Stdout = &combined
	wrapped.Stderr = &combined

	if err := wrapped.Start(); err != nil {
		return combined.Bytes(), err
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- wrapped.Wait()
	}()

	select {
	case err := <-waitDone:
		// CommandContext may win the race and surface "signal: killed" on the
		// completion path when the deadline fires; still treat that as timeout
		// and ensure the process group (git-remote-*/ssh) is torn down.
		if runCtx.Err() != nil {
			killCmdProcessGroup(wrapped.Process)
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) && timeoutSec > 0 {
				return combined.Bytes(), &FetchTimedOutError{Seconds: timeoutSec}
			}
			return combined.Bytes(), runCtx.Err()
		}
		if err != nil {
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) && timeoutSec > 0 {
				return combined.Bytes(), &FetchTimedOutError{Seconds: timeoutSec}
			}
			if runCtx.Err() != nil {
				return combined.Bytes(), runCtx.Err()
			}
			return combined.Bytes(), err
		}
		return combined.Bytes(), nil
	case <-runCtx.Done():
		killCmdProcessGroup(wrapped.Process)
		waitErr := <-waitDone
		if waitErr == nil {
			return combined.Bytes(), nil
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) && timeoutSec > 0 {
			return combined.Bytes(), &FetchTimedOutError{Seconds: timeoutSec}
		}
		return combined.Bytes(), runCtx.Err()
	}
}
