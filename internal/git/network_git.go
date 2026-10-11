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

// RunNetworkGit runs a git command that may contact a remote. It applies
// PrepareNetworkGit, honors parent cancellation, and when timeoutSec > 0 enforces
// a per-attempt deadline. On timeout the whole process group is killed so
// git-remote-https and ssh grandchildren do not linger.
func RunNetworkGit(parent context.Context, cmd *exec.Cmd, timeoutSec int) ([]byte, error) {
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
		<-waitDone
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) && timeoutSec > 0 {
			return combined.Bytes(), &FetchTimedOutError{Seconds: timeoutSec}
		}
		return combined.Bytes(), runCtx.Err()
	}
}
