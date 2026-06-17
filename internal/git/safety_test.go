package git_test

import (
	"strings"
	"testing"

	"github.com/git-fire/git-harness/safety"
)

func TestHarnessSanitizeText_FreezeMessagesPreserved(t *testing.T) {
	msgs := []string{
		"could not reach remote — check your network and try again",
		"could not authenticate with remote — check your credentials and try again",
		"fetch did not complete — try again when the remote is reachable",
	}
	for _, msg := range msgs {
		got := safety.SanitizeText(msg)
		if got != msg {
			t.Errorf("freeze message was modified:\n  input: %q\n  got:   %q", msg, got)
		}
	}
}

func TestHarnessSanitizeText_RedactsCredentials(t *testing.T) {
	input := "fatal: https://user:supersecret@github.com/org/repo.git"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "supersecret") {
		t.Errorf("password not redacted: %q", got)
	}
}
