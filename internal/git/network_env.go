package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	defaultSSHConnectTimeoutSec = 30
	defaultHTTPLowSpeedLimit    = "1000"
	defaultHTTPLowSpeedTimeSec  = "30"
)

func applyFailFastNetworkEnv(cmd *exec.Cmd) {
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	env = setEnvIfUnset(env, "GIT_HTTP_LOW_SPEED_LIMIT", defaultHTTPLowSpeedLimit)
	env = setEnvIfUnset(env, "GIT_HTTP_LOW_SPEED_TIME", defaultHTTPLowSpeedTimeSec)

	// GIT_SSH_COMMAND overrides GIT_SSH; only inject when neither is set and
	// core.sshCommand is unset (same preserve path as an explicit SSH wrapper).
	if !envHasPrefix(env, "GIT_SSH_COMMAND=") && !envHasPrefix(env, "GIT_SSH=") && !repoHasCoreSSHCommand(cmd.Dir, env) {
		sshCmd := fmt.Sprintf("ssh -o BatchMode=yes -o ConnectTimeout=%d", defaultSSHConnectTimeoutSec)
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
	}
	cmd.Env = env
}

func setEnvIfUnset(env []string, key, value string) []string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return env
		}
	}
	return append(env, prefix+value)
}

func envHasPrefix(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

func repoHasCoreSSHCommand(repoDir string, env []string) bool {
	if strings.TrimSpace(repoDir) == "" {
		return false
	}
	probe := exec.Command("git", "-C", repoDir, "config", "--get", "core.sshCommand")
	probe.Env = env
	out, err := probe.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
