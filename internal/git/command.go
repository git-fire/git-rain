package git

import (
	"os"
	"os/exec"
	"strings"
)

const gitTerminalPromptKey = "GIT_TERMINAL_PROMPT="

// nonInteractiveGitEnv returns a copy of env with GIT_TERMINAL_PROMPT=0 so git
// never prompts for credentials on the controlling TTY during batch operations.
func nonInteractiveGitEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, gitTerminalPromptKey) {
			continue
		}
		out = append(out, e)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0")
}

// PrepareNetworkGit configures cmd for fetch/push operations that may contact
// a remote. Callers must still set cmd.Dir (and stdout/stderr as needed).
func PrepareNetworkGit(cmd *exec.Cmd) {
	cmd.Env = nonInteractiveGitEnv(cmd.Env)
}
