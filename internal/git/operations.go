package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/git-rain/git-rain/internal/safety"
)

// Worktree represents a git worktree
type Worktree struct {
	Path   string // Absolute path to worktree
	Branch string // Current branch in this worktree
	Head   string // Current HEAD SHA
	IsMain bool   // True if this is the main worktree
}

// getCommitSHA returns the SHA of a commit ref
func getCommitSHA(repoPath, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed for %s: %w", ref, err)
	}

	sha := strings.TrimSpace(string(output))
	return sha, nil
}

// GetCommitSHA returns the SHA for a ref in the repository.
func GetCommitSHA(repoPath, ref string) (string, error) {
	return getCommitSHA(repoPath, ref)
}

// GetCurrentBranch returns the currently checked out branch
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current failed: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("not on any branch (detached HEAD?)")
	}

	return branch, nil
}

// HasStagedChanges checks if there are staged changes
func HasStagedChanges(repoPath string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = repoPath

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet failed: %w", err)
	}

	return false, nil
}

// HasUnstagedChanges checks if there are unstaged changes (including untracked files)
func HasUnstagedChanges(repoPath string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	cmd.Dir = repoPath

	err := cmd.Run()
	hasModified := false
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			hasModified = true
		} else {
			return false, fmt.Errorf("git diff --quiet failed: %w", err)
		}
	}

	cmd = exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git ls-files failed: %w", err)
	}

	hasUntracked := len(strings.TrimSpace(string(output))) > 0

	return hasModified || hasUntracked, nil
}

// ListWorktrees returns all worktrees for a repository
func ListWorktrees(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var worktrees []Worktree
	var current Worktree
	isFirst := true

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				current.IsMain = isFirst
				worktrees = append(worktrees, current)
				current = Worktree{}
				isFirst = false
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "worktree":
			current.Path = value
		case "HEAD":
			current.Head = value
		case "branch":
			branch := strings.TrimPrefix(value, "refs/heads/")
			current.Branch = branch
		}
	}

	if current.Path != "" {
		current.IsMain = isFirst
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func commandError(action string, err error, output []byte) error {
	out := strings.TrimSpace(string(output))
	if out == "" {
		return fmt.Errorf("%s failed: %w", action, err)
	}
	return fmt.Errorf("%s failed: %w (stderr: %s)", action, err, safety.SanitizeText(out))
}
