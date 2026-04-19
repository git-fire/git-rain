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

func TestGardenSkySeedsForBatchCapped(t *testing.T) {
	rb := NewRainBackground(100, 6, config.UIRainAnimationGarden)
	rb.Garden.SeedSpawnRate = 0.35
	const need = 200
	for range 400 {
		k := rb.gardenSkySeedsForBatch(need)
		if k > 12 {
			t.Fatalf("gardenSkySeedsForBatch capped too loose: k=%d for need=%d", k, need)
		}
	}
}

func TestGardenSkySeedsThrottleByMaturityAndFlight(t *testing.T) {
	rb := NewRainBackground(24, 6, config.UIRainAnimationGarden)
	rb.Garden.SeedSpawnRate = 0.25
	// Young garden: relief ~0 → tight flying cap; many seeds already aloft → no new sky seeds.
	for i := range rb.GardenPlots {
		rb.GardenPlots[i] = gardenPlot{stage: gardenStageSprout}
	}
	for i := 0; i < 8; i++ {
		rb.Drops = append(rb.Drops, RainDrop{IsSeed: true, MaxAge: 99, Y: 2, X: i % rb.Width})
	}
	if k := rb.gardenSkySeedsForBatch(40); k != 0 {
		t.Fatalf("expected 0 new sky seeds when flying count exceeds young cap, got %d", k)
	}
	// Mature garden: higher relief and ceiling → room for more seeds if few in flight.
	rb.Drops = nil
	for i := range rb.GardenPlots {
		rb.GardenPlots[i] = gardenPlot{stage: gardenStageBloom}
	}
	cap := rb.gardenMaxFlyingSkySeeds(rb.gardenSeedThrottleRelief())
	if cap < 6 {
		t.Fatalf("expected mature garden to allow a generous flying-seed cap, got %d", cap)
	}
	if k := rb.gardenSkySeedsForBatch(40); k < 1 {
		t.Fatalf("expected at least one sky seed when mature and sky is clear, got %d", k)
	}
}

func TestApplyGardenStormWallClockScaleWiderAndRareSlower(t *testing.T) {
	cfgNormal := &config.Config{}
	narrow := ResolveGardenTuning(GardenTuning{})
	wide := narrow
	applyGardenStormWallClockScale(&narrow, cfgNormal, 150, 40)
	applyGardenStormWallClockScale(&wide, cfgNormal, 150, 100)
	if wide.PlantedToSproutMoisture <= narrow.PlantedToSproutMoisture {
		t.Fatalf("wider garden width should increase moisture thresholds (got narrow=%d wide=%d)",
			narrow.PlantedToSproutMoisture, wide.PlantedToSproutMoisture)
	}

	cfgRare := &config.Config{}
	cfgRare.UI.GardenSeedRate = 0.06
	rare := ResolveGardenTuning(GardenTuning{SeedSpawnRate: 0.06})
	applyGardenStormWallClockScale(&rare, cfgRare, 150, 56)
	if rare.PlantedToSproutMoisture <= narrow.PlantedToSproutMoisture {
		t.Fatalf("rare seed preset should slow growth vs normal (rare=%d narrow=%d)",
			rare.PlantedToSproutMoisture, narrow.PlantedToSproutMoisture)
	}
	if rare.SeedSpawnRate >= narrow.SeedSpawnRate {
		t.Fatalf("rare wall-clock scale should lower effective seed rate (rare=%v narrow=%v)",
			rare.SeedSpawnRate, narrow.SeedSpawnRate)
	}
}

func TestResolveGardenTuningFillsZeroDefaults(t *testing.T) {
	d := DefaultGardenTuning()
	got := ResolveGardenTuning(GardenTuning{})
	if got != d {
		t.Fatalf("zero tuning should equal defaults\n got: %#v\nwant: %#v", got, d)
	}

	got = ResolveGardenTuning(GardenTuning{
		PlantedToSproutMoisture: 99,
		OffspringMin:            5,
		OffspringMax:            2, // out of order; resolver should clamp up
	})
	if got.PlantedToSproutMoisture != 99 {
		t.Fatalf("user override not preserved: got %d", got.PlantedToSproutMoisture)
	}
	if got.OffspringMin != 5 || got.OffspringMax < got.OffspringMin {
		t.Fatalf("offspring bounds wrong: min=%d max=%d", got.OffspringMin, got.OffspringMax)
	}
	if got.SeedSpawnRate != d.SeedSpawnRate {
		t.Fatalf("unset SeedSpawnRate should fall back to default %v, got %v", d.SeedSpawnRate, got.SeedSpawnRate)
	}
}

func TestGardenWaterCapPerColumnPerFrame(t *testing.T) {
	const w, h = 8, 6
	rb := NewRainBackground(w, h, config.UIRainAnimationGarden)
	// Disable extra-absorb so the cap is hard. The resolver treats zero as
	// "unset" by design (matches the rest of the TOML config), so we override
	// the field directly after resolving.
	rb.Garden.RainAbsorbExtraChance = 0
	rb.Drops = nil
	col := 3
	rb.GardenPlots[col] = gardenPlot{stage: gardenStagePlanted}
	soilY := h - 2
	// Pile many rain drops onto the same soil cell within one frame.
	for i := 0; i < 20; i++ {
		rb.Drops = append(rb.Drops, RainDrop{
			X:      col,
			Y:      soilY,
			Char:   "·",
			MaxAge: h * 4,
			Speed:  1, // ensure they advance and trigger soil-band logic
		})
	}
	startMoisture := rb.GardenPlots[col].moisture
	rb.Update()
	got := rb.GardenPlots[col].moisture - startMoisture
	if got > 1 {
		t.Fatalf("water cap broken: column gained %d moisture in one frame, want <=1", got)
	}
	if got < 1 {
		t.Fatalf("expected at least 1 moisture point from many drops, got %d", got)
	}
}

func TestGardenDeathScattersOffspring(t *testing.T) {
	const w, h = 24, 6
	rb := NewRainBackground(w, h, config.UIRainAnimationGarden)
	rb.SetGardenTuning(GardenTuning{
		OffspringMin:    2,
		OffspringMax:    3,
		OffspringSpread: 4,
	})
	rb.Drops = nil
	origin := 12
	// Park a plot on the verge of death.
	rb.GardenPlots[origin] = gardenPlot{
		stage:     gardenStageWither,
		witherAge: rb.Garden.WitherDuration - 1,
	}
	rb.gardenAdvancePlots()

	if rb.GardenPlots[origin].stage != gardenStageNone {
		t.Fatalf("expected plot to transition to None on death, got %d", rb.GardenPlots[origin].stage)
	}
	seedCount := 0
	for _, d := range rb.Drops {
		if !d.IsSeed {
			continue
		}
		seedCount++
		offset := d.X - origin
		if offset < -rb.Garden.OffspringSpread || offset > rb.Garden.OffspringSpread {
			t.Fatalf("seed at x=%d landed outside spread window around origin %d", d.X, origin)
		}
	}
	if seedCount < rb.Garden.OffspringMin || seedCount > rb.Garden.OffspringMax {
		t.Fatalf("expected %d-%d offspring seeds, got %d", rb.Garden.OffspringMin, rb.Garden.OffspringMax, seedCount)
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
