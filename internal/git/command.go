package git

import (
	"os/exec"

	harnessgit "github.com/git-fire/git-harness/git"
)

// PrepareNetworkGit configures cmd for fetch/push operations that may contact
// a remote. Delegates to git-harness for GIT_TERMINAL_PROMPT=0, then adds
// fail-fast SSH and HTTP env unless the user already set GIT_SSH_COMMAND,
// GIT_HTTP_LOW_SPEED_*, or core.sshCommand in the repo.
func PrepareNetworkGit(cmd *exec.Cmd) {
	harnessgit.PrepareNetworkGit(cmd)
	applyFailFastNetworkEnv(cmd)
}
