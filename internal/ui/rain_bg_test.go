package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

func TestRainBackgroundMatrixRenderLineWidths(t *testing.T) {
	const w, h = 24, 5
	rb := NewRainBackground(w, h, config.UIRainAnimationMatrix, nil)
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

func TestMatrixMarqueeCharSingleCell(t *testing.T) {
	for frame := 0; frame < 2000; frame++ {
		for x := 0; x < 80; x++ {
			if c, ok := matrixMarqueeChar(x, frame, 80); ok {
				if got := lipgloss.Width(c); got != 1 {
					t.Fatalf("marquee char width %d at x=%d frame=%d: %q", got, x, frame, c)
				}
			}
		}
	}
}

func TestMatrixVerticalSubliminalCharSingleCell(t *testing.T) {
	for frame := 0; frame < 500; frame++ {
		if c, ok := matrixVerticalSubliminalChar(frame); ok {
			if got := lipgloss.Width(c); got != 1 {
				t.Fatalf("vertical subliminal width %d at frame=%d: %q", got, frame, c)
			}
		}
	}
}

func TestRainBackgroundAdvancedAccumulatesFlowerMoisture(t *testing.T) {
	rb := NewRainBackground(40, 5, config.UIRainAnimationAdvanced, nil)
	for i := 0; i < 400; i++ {
		rb.Update()
	}
	maxD := 0
	for _, f := range rb.Flowers {
		if f.drops > maxD {
			maxD = f.drops
		}
	}
	if maxD < 10 {
		t.Fatalf("expected substantial moisture accumulation, got max drops=%d", maxD)
	}
}

func TestRainBackgroundGardenFastPresetReachesFullBloomQuickly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.UI.RainAnimationMode = config.UIRainAnimationGarden
	cfg.UI.GardenBloomPreset = config.UIGardenBloomFast
	rb := NewRainBackground(24, 5, config.UIRainAnimationGarden, &cfg)
	for i := 0; i < 120; i++ {
		rb.Update()
	}
	found := false
	for x := 0; x < rb.Width; x++ {
		if rb.flowerStage(x) == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected full bloom on fast preset within 120 frames")
	}
}

func TestRainBackgroundGardenTightMoistureCapReachesFullBloom(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.UI.RainAnimationMode = config.UIRainAnimationGarden
	cfg.UI.GardenBloomPreset = config.UIGardenBloomNormal
	cfg.UI.GardenMoistureCap = config.UIGardenMoistureTight
	rb := NewRainBackground(40, 5, config.UIRainAnimationGarden, &cfg)
	for i := 0; i < 800; i++ {
		rb.Update()
	}
	found := false
	for x := 0; x < rb.Width; x++ {
		if rb.flowerStage(x) == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one column to reach full bloom with tight moisture cap")
	}
}
