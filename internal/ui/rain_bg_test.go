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
	s := RenderRainWave(width, 7, config.UIRainAnimationMatrix, false)
	if got := lipgloss.Width(s); got != width {
		t.Fatalf("lipgloss.Width(RenderRainWave matrix) = %d, want %d", got, width)
	}
}

func TestRenderRainWaveGardenWidths(t *testing.T) {
	const width = 40
	for _, sunny := range []bool{false, true} {
		s := RenderRainWave(width, 11, config.UIRainAnimationGarden, sunny)
		if got := lipgloss.Width(s); got != width {
			t.Fatalf("garden sunny=%v: lipgloss.Width = %d, want %d", sunny, got, width)
		}
	}
}

func TestRainBackgroundGardenRenderLineWidths(t *testing.T) {
	const w, h = 28, 5
	rb := NewRainBackground(w, h, config.UIRainAnimationGarden)
	for i := 0; i < 30; i++ {
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

func TestGardenBackgroundFinishesStorm(t *testing.T) {
	const w, h = 32, 5
	rb := NewRainBackground(w, h, config.UIRainAnimationGarden)
	rb.GardenSunny = false
	for x := 0; x < w; x++ {
		rb.GardenPlots[x].stage = gardenStageEternal
	}
	rb.gardenMaybeFinishStorm()
	if !rb.GardenSunny {
		t.Fatal("expected storm to end when garden is visually full")
	}
	if len(rb.Drops) != 0 {
		t.Fatalf("expected drops cleared, got %d", len(rb.Drops))
	}
}
