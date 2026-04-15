package safety_test

import (
	"strings"
	"testing"

	"github.com/git-rain/git-rain/internal/safety"
)

func TestSanitizeText_CleanInput(t *testing.T) {
	input := "fetch did not complete — try again when the remote is reachable"
	got := safety.SanitizeText(input)
	if got != input {
		t.Errorf("clean input was modified:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestSanitizeText_HTTPSCredentials(t *testing.T) {
	input := "fatal: https://user:supersecret@github.com/org/repo.git"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "supersecret") {
		t.Errorf("password not redacted: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker in output: %q", got)
	}
}

func TestSanitizeText_GitHubPersonalAccessToken(t *testing.T) {
	input := "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef1234"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef1234") {
		t.Errorf("GitHub PAT not redacted: %q", got)
	}
}

func TestSanitizeText_AWSAccessKey(t *testing.T) {
	input := "AKIAJSIE27OOBC3BZMAA is your key"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "AKIAJSIE27OOBC3BZMAA") {
		t.Errorf("AWS access key not redacted: %q", got)
	}
}

func TestSanitizeText_GenericPassword(t *testing.T) {
	input := "password=hunter2abc"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "hunter2abc") {
		t.Errorf("password value not redacted: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output: %q", got)
	}
}

func TestSanitizeText_GitLabToken(t *testing.T) {
	input := "glpat-abcdefghijklmnopqrstu"
	got := safety.SanitizeText(input)
	if strings.Contains(got, "abcdefghijklmnopqrstu") {
		t.Errorf("GitLab token not redacted: %q", got)
	}
}

func TestSanitizeText_EmptyString(t *testing.T) {
	got := safety.SanitizeText("")
	if got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestSanitizeText_FreezeMessage_Preserved(t *testing.T) {
	// Freeze / network error messages should pass through unchanged
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
