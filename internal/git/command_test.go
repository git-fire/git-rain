package git

import (
	"os/exec"
	"strings"
	"testing"

	testutil "github.com/git-fire/git-testkit"
)

func TestNonInteractiveGitEnv_OverridesExistingPrompt(t *testing.T) {
	env := nonInteractiveGitEnv([]string{
		"HOME=/tmp",
		"GIT_TERMINAL_PROMPT=1",
		"PATH=/bin",
	})
	if !containsEnv(env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("expected GIT_TERMINAL_PROMPT=0 in env, got %#v", env)
	}
	for _, e := range env {
		if e == "GIT_TERMINAL_PROMPT=1" {
			t.Fatalf("did not override existing GIT_TERMINAL_PROMPT: %#v", env)
		}
	}
}

func TestPrepareNetworkGit_SetsEnvOnCommand(t *testing.T) {
	cmd := exec.Command("git", "version")
	PrepareNetworkGit(cmd)
	if !containsEnv(cmd.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("prepareNetworkGit did not set GIT_TERMINAL_PROMPT=0: %#v", cmd.Env)
	}
}

func TestFetchFailureReason_TerminalPromptsDisabled(t *testing.T) {
	got := fetchFailureReason([]byte("fatal: could not read Username for 'https://github.com': terminal prompts disabled"))
	want := "could not authenticate with remote — check your credentials and try again"
	if got != want {
		t.Fatalf("fetchFailureReason() = %q, want %q", got, want)
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

func containsEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}
