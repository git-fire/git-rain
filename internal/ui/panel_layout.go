package ui

// Horizontal layout for the main Bubble Tea panel (must stay consistent across
// repo list, ignored list, settings, rain banner, and PathWidthFor).

// panelOuterMarginTotal is the number of terminal columns reserved outside the
// bordered panel so the frame does not touch the left/right edge.
const panelOuterMarginTotal = 6

// panelBoxHorizontalPadding is the sum of left and right padding on boxStyle
// (Padding(1, 2) => 2 + 2). Keep in sync with boxStyle in repo_selector.go and
// applyColorProfile in color_profiles.go.
const panelBoxHorizontalPadding = 4

// PanelBlockWidth returns the lipgloss Width passed to boxStyle for the main panel.
func PanelBlockWidth(terminalWidth int) int {
	w := terminalWidth - panelOuterMarginTotal
	if w < 0 {
		return 0
	}
	return w
}

// PanelTextWidth is the maximum cell width for one line of content inside the
// panel after horizontal padding (use for clamping, PathWidthFor, MaxWidth).
func PanelTextWidth(terminalWidth int) int {
	w := PanelBlockWidth(terminalWidth) - panelBoxHorizontalPadding
	if w < 0 {
		return 0
	}
	return w
}

// RainDisplayWidth caps the ASCII rain banner width to match the text area.
func RainDisplayWidth(terminalWidth int) int {
	tw := PanelTextWidth(terminalWidth)
	if tw > 70 {
		return 70
	}
	return tw
}
