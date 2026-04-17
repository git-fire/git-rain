package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

func TestRainBackgroundMatrixRenderLineWidths(t *testing.T) {
	const w, h = 24, 5
	rb := NewRainBackground(w, h, config.UIRainAnimationMatrix)
	for i := 0; i < 20; i++ {
		rb.Update()
	}
	out := rb.Render()
	lines := strings.Split(out, "\n")
	if len(lines) != h {
		t.Fatalf("expected %d lines, got %d", h, len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != w {
			t.Fatalf("line %d: lipgloss.Width = %d, want %d\n%q", i, got, w, line)
		}
	}
}

func TestRenderRainWaveMatrixWidth(t *testing.T) {
	const width = 40
	s := RenderRainWave(width, 7, config.UIRainAnimationMatrix)
	if got := lipgloss.Width(s); got != width {
		t.Fatalf("lipgloss.Width(RenderRainWave matrix) = %d, want %d", got, width)
	}
}
