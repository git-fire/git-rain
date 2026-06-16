package git

import (
	"os/exec"

	harnessgit "github.com/git-fire/git-harness/git"
)

// PrepareNetworkGit configures cmd for fetch/push operations that may contact
// a remote. Delegates to git-harness so credential behavior stays aligned with git-fire.
func PrepareNetworkGit(cmd *exec.Cmd) {
	harnessgit.PrepareNetworkGit(cmd)
}
