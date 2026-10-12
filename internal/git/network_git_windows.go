//go:build windows

package git

import (
	"os"
	"os/exec"
)

func setCmdProcessGroup(cmd *exec.Cmd) {
	// Process groups for killing grandchildren are not wired on Windows yet.
}

func killCmdProcessGroup(proc *os.Process) {
	if proc == nil {
		return
	}
	_ = proc.Kill()
}
