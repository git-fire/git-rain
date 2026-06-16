package git

import (
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
