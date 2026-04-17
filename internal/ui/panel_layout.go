package ui

import "github.com/charmbracelet/lipgloss"

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

// RainDisplayWidth is the cell width for the rain background and wave strip.
// It must equal PanelTextWidth so every line inside the bordered panel matches
// the inner content width; otherwise lipgloss rounded borders show gaps when
// the first rows are shorter than later rows (main menu and settings).
func RainDisplayWidth(terminalWidth int) int {
	return PanelTextWidth(terminalWidth)
}

// panelInnerLipglossWidth is the lipgloss "width" setting for the block *inside*
// the main panel's border and horizontal padding. alignTextHorizontal pads each
// line to this width before applyBorder measures line widths — if any physical
// line is wider in the terminal (e.g. emoji wcwidth 2 vs ansi width 1), the
// border row becomes shorter than the content and corners look like gaps.
func panelInnerLipglossWidth(innerBlockWidth int) int {
	if innerBlockWidth <= 0 {
		return 0
	}
	w := innerBlockWidth - panelBoxHorizontalPadding
	if w < 1 {
		return 1
	}
	return w
}

// renderMainPanelBox renders `inner` (no outer border) inside boxStyle with a
// consistent inner width. A pre-pass forces every line to exactly
// panelInnerLipglossWidth(innerBlockWidth) lipgloss cells so border segments
// match the terminal-rendered content on desktops with wide emoji.
func renderMainPanelBox(innerBlockWidth int, inner string) string {
	if innerBlockWidth <= 0 {
		return boxStyle.Render(inner)
	}
	cells := panelInnerLipglossWidth(innerBlockWidth)
	normalized := lipgloss.NewStyle().Width(cells).Render(inner)
	return boxStyle.Width(innerBlockWidth).Render(normalized)
}
