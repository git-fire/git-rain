package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func TestFetchFailureReason_TerminalPromptsDisabled(t *testing.T) {
	got := fetchFailureReason([]byte("fatal: could not read Username for 'https://github.com': terminal prompts disabled"))
	want := "could not authenticate with remote — check your credentials and try again"
	if got != want {
		t.Fatalf("fetchFailureReason() = %q, want %q", got, want)
	}
}

func isolatedPrepareNetworkEnv(t *testing.T) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	}
}

func TestPrepareNetworkGit_SetsFailFastEnv(t *testing.T) {
	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{Name: "failfast-default"})
	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	cmd.Env = isolatedPrepareNetworkEnv(t)
	PrepareNetworkGit(cmd)

	env := strings.Join(cmd.Env, "\n")
	if !strings.Contains(env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("expected GIT_TERMINAL_PROMPT=0 in env")
	}
	if !strings.Contains(env, "GIT_HTTP_LOW_SPEED_LIMIT="+defaultHTTPLowSpeedLimit) {
		t.Fatalf("expected HTTP low-speed limit in env, got %q", env)
	}
	if !strings.Contains(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes") {
		t.Fatalf("expected batch-mode GIT_SSH_COMMAND in env, got %q", env)
	}
}

func TestPrepareNetworkGit_PreservesUserGitSSHCommand(t *testing.T) {
	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{Name: "failfast-user-ssh"})
	t.Setenv("GIT_SSH_COMMAND", "ssh -o IdentityFile=/tmp/custom")

	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	cmd.Env = append(isolatedPrepareNetworkEnv(t), "GIT_SSH_COMMAND=ssh -o IdentityFile=/tmp/custom")
	PrepareNetworkGit(cmd)

	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") && !strings.Contains(e, "IdentityFile=/tmp/custom") {
			t.Fatalf("clobbered user GIT_SSH_COMMAND: %s", e)
		}
	}
}

func TestPrepareNetworkGit_PreservesCoreSSHCommand(t *testing.T) {
	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{Name: "failfast-core-ssh"})
	runGit := exec.Command("git", "-C", repo, "config", "core.sshCommand", "ssh -o IdentityFile=/tmp/from-core")
	if out, err := runGit.CombinedOutput(); err != nil {
		t.Fatalf("git config core.sshCommand: %v (%s)", err, out)
	}

	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	cmd.Env = isolatedPrepareNetworkEnv(t)
	PrepareNetworkGit(cmd)

	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			t.Fatalf("should not set GIT_SSH_COMMAND when core.sshCommand is configured, got %s", e)
		}
	}
}

func TestPrepareNetworkGit_PreservesUserHTTPLowSpeed(t *testing.T) {
	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{Name: "failfast-http"})
	t.Setenv("GIT_HTTP_LOW_SPEED_LIMIT", "42")
	t.Setenv("GIT_HTTP_LOW_SPEED_TIME", "99")

	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	cmd.Env = append(isolatedPrepareNetworkEnv(t),
		"GIT_HTTP_LOW_SPEED_LIMIT=42",
		"GIT_HTTP_LOW_SPEED_TIME=99",
	)
	PrepareNetworkGit(cmd)

	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "GIT_HTTP_LOW_SPEED_LIMIT=") && !strings.Contains(e, "=42") {
			t.Fatalf("clobbered GIT_HTTP_LOW_SPEED_LIMIT: %s", e)
		}
	}
}

func TestNetworkFetch_UnauthenticatedHTTPSFailsWithoutPrompt(t *testing.T) {
	repo := testutil.CreateTestRepo(t, testutil.RepoOptions{
		Name: "https-fetch-repo",
		Remotes: map[string]string{
			"origin": "https://github.com/git-rain/nonexistent-repo-auth-test.git",
		},
	})

	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = repo
	PrepareNetworkGit(cmd)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected fetch to fail without credentials")
	}
	msg := strings.ToLower(string(output) + " " + err.Error())
	if !strings.Contains(msg, "terminal prompts disabled") &&
		!strings.Contains(msg, "could not read username") {
		t.Fatalf("expected non-interactive auth failure, got output=%q err=%v", strings.TrimSpace(string(output)), err)
	}
}
