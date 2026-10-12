package cmd

import (
	"testing"
)

func TestAgentNonInteractive_FlagAndEnv(t *testing.T) {
	resetFlags()
	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "")
	baseline := agentNonInteractive()
	if baseline != !stdinIsTerminal() {
		t.Fatalf("baseline non-interactive=%v stdinIsTerminal=%v", baseline, stdinIsTerminal())
	}

	rainNonInteractive = true
	if !agentNonInteractive() {
		t.Fatal("expected non-interactive when --non-interactive is set")
	}
	rainNonInteractive = false

	t.Setenv("GIT_RAIN_NON_INTERACTIVE", "1")
	if !agentNonInteractive() {
		t.Fatal("expected non-interactive when GIT_RAIN_NON_INTERACTIVE=1")
	}
}
