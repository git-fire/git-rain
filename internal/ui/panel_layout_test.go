package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestPanelLayoutMatchesBoxStylePadding keeps panelBoxHorizontalPadding in sync
// with the main panel lipgloss style (Border + Padding(1,2)).
func TestPanelLayoutMatchesBoxStylePadding(t *testing.T) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)
	if got := box.GetHorizontalPadding(); got != panelBoxHorizontalPadding {
		t.Fatalf("lipgloss horizontal padding = %d, panelBoxHorizontalPadding = %d — update one to match",
			got, panelBoxHorizontalPadding)
	}
}

func TestPanelBlockAndTextWidth(t *testing.T) {
	if got := PanelBlockWidth(120); got != 114 {
		t.Fatalf("PanelBlockWidth(120) = %d, want 114", got)
	}
	if got := PanelTextWidth(120); got != 110 {
		t.Fatalf("PanelTextWidth(120) = %d, want 110", got)
	}
	if got := PanelTextWidth(6); got != 0 {
		t.Fatalf("PanelTextWidth(6) = %d, want 0", got)
	}
	if got := RainDisplayWidth(200); got != 70 {
		t.Fatalf("RainDisplayWidth(200) = %d, want 70", got)
	}
	if got := RainDisplayWidth(80); got != 70 {
		t.Fatalf("RainDisplayWidth(80) = %d, want 70", got)
	}
}
