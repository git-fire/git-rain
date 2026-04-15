package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// pkgMu serialises package-managed registry operations within a single process.
var pkgMu sync.RWMutex

// LockPath returns the cross-process lock file path for a registry file.
func LockPath(registryPath string) string {
	return registryPath + ".lock"
}

// LockFileInfo describes an existing registry lock file on disk.
type LockFileInfo struct {
	LockPath          string
	PID               int
	OwnerAppearsAlive bool
}

// ReadLockFile returns information about the registry lock file if it exists.
func ReadLockFile(registryPath string) (*LockFileInfo, error) {
	lp := LockPath(registryPath)
	if _, err := os.Stat(lp); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(lp)
	info := &LockFileInfo{LockPath: lp}
	if err != nil {
		return info, fmt.Errorf("reading registry lock: %w", err)
	}
	var pid int
	if _, scanErr := fmt.Sscan(string(data), &pid); scanErr != nil || pid <= 0 {
		info.PID = 0
		return info, nil
	}
	info.PID = pid
	info.OwnerAppearsAlive = processAppearsAlive(pid)
	return info, nil
}

// RemoveLockFile removes the registry lock file if present.
func RemoveLockFile(registryPath string) error {
	return os.Remove(LockPath(registryPath))
}

func processAppearsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone)
}

const lockTimeout = 5 * time.Second
const lockPollInterval = 50 * time.Millisecond

func acquireLock(registryPath string) (release func(), err error) {
	lockPath := LockPath(registryPath)

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return func() {}, fmt.Errorf("creating registry lock directory: %w", err)
	}

	deadline := time.Now().Add(lockTimeout)

	for {
		f, createErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}

		if !os.IsExist(createErr) {
			return func() {}, fmt.Errorf("acquiring registry lock: %w", createErr)
		}

		if staleLock(lockPath) {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: removing stale registry lock (owner process is gone): %s\n", lockPath)
			_ = os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			return func() {}, fmt.Errorf("timed out waiting for registry lock %s", lockPath)
		}

		time.Sleep(lockPollInterval)
	}
}

func staleLock(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscan(string(data), &pid); err != nil || pid <= 0 {
		return false
	}
	return !processAppearsAlive(pid)
}
