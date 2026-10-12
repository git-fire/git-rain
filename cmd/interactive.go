package cmd

import (
	"os"
	"strings"
)

// stdinIsTerminal reports whether stdin is an interactive character device.
func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// agentNonInteractive is true when the run must not prompt or require a TUI.
// Precedence: --non-interactive, GIT_RAIN_NON_INTERACTIVE, non-TTY stdin.
func agentNonInteractive() bool {
	if rainNonInteractive {
		return true
	}
	v := strings.TrimSpace(os.Getenv("GIT_RAIN_NON_INTERACTIVE"))
	if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		return true
	}
	return !stdinIsTerminal()
}
