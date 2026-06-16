package git

import (
	"fmt"
	"strings"

	harnessgit "github.com/git-fire/git-harness/git"
	harnessafety "github.com/git-fire/git-harness/safety"
)

// Worktree is a git worktree (delegates to git-harness).
type Worktree = harnessgit.Worktree

func getCommitSHA(repoPath, ref string) (string, error) {
	return harnessgit.GetCommitSHA(repoPath, ref)
}

// GetCommitSHA returns the SHA for a ref in the repository.
func GetCommitSHA(repoPath, ref string) (string, error) {
	return harnessgit.GetCommitSHA(repoPath, ref)
}

// GetCurrentBranch returns the currently checked out branch.
func GetCurrentBranch(repoPath string) (string, error) {
	return harnessgit.GetCurrentBranch(repoPath)
}

// HasStagedChanges checks if there are staged changes.
func HasStagedChanges(repoPath string) (bool, error) {
	return harnessgit.HasStagedChanges(repoPath)
}

// HasUnstagedChanges checks if there are unstaged changes (including untracked files).
func HasUnstagedChanges(repoPath string) (bool, error) {
	return harnessgit.HasUnstagedChanges(repoPath)
}

// ListWorktrees returns all worktrees for a repository.
func ListWorktrees(repoPath string) ([]Worktree, error) {
	return harnessgit.ListWorktrees(repoPath)
}

func commandError(action string, err error, output []byte) error {
	out := strings.TrimSpace(string(output))
	if out == "" {
		return fmt.Errorf("%s failed: %w", action, err)
	}
	return fmt.Errorf("%s failed: %w (stderr: %s)", action, err, harnessafety.SanitizeText(out))
}
