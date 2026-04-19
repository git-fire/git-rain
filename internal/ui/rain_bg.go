package ui

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

var rainDropChars = [...]string{"│", "╵", "·", "˙", "╷", "⁚", "⋮"}

// matrixGlyphPool is half-width katakana + digits + symbols for a Matrix-style fall.
var matrixGlyphPool = [...]string{
	"ｱ", "ｲ", "ｳ", "ｴ", "ｵ", "ｶ", "ｷ", "ｸ", "ｹ", "ｺ", "ｻ", "ｼ", "ｽ", "ｾ", "ｿ",
	"ﾀ", "ﾁ", "ﾂ", "ﾃ", "ﾄ", "ﾅ", "ﾆ", "ﾇ", "ﾈ", "ﾉ", "ﾊ", "ﾋ", "ﾌ", "ﾍ", "ﾎ",
	"ﾏ", "ﾐ", "ﾑ", "ﾒ", "ﾓ", "ﾔ", "ﾕ", "ﾖ", "ﾗ", "ﾘ", "ﾙ", "ﾚ", "ﾛ", "ﾜ", "ﾝ",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	":", ";", "<", ">", "?", "@", "#", "$", "%", "^", "&", "*", "+", "=",
}

// cloudChars for advanced mode top row
var cloudChars = [...]string{"☁", "░", "▒", "▓", "█"}

// flowerStages for advanced mode bottom row: growth over time
var flowerStages = []string{"·", "♦", "✿", "❀"}

// gardenFlowerAccentHex holds distinct flower-head colors (garden mode).
var gardenFlowerAccentHex = []string{
	"#EC407A", // pink
	"#AB47BC", // purple
	"#FFCA28", // amber
	"#FF7043", // deep orange
	"#BA68C8", // lavender
	"#F06292", // light pink
	"#FFD54F", // golden yellow
	"#7E57C2", // deep purple
	"#26C6DA", // cyan
	"#FFEE58", // yellow
	"#EF5350", // coral
	"#66BB6A", // green blossom
}

func gardenFlowerForeground(tint int) lipgloss.Color {
	n := len(gardenFlowerAccentHex)
	if n == 0 {
		return activeProfile().flowerColor
	}
	u := tint % n
	if u < 0 {
		u += n
	}
	return lipgloss.Color(gardenFlowerAccentHex[u])
}

func gardenFlowerStyle(tint int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(gardenFlowerForeground(tint))
}

// RainDrop represents a single falling raindrop particle
type RainDrop struct {
	X        int
	Y        int
	Char     string
	ColorIdx int
	Age      int
	MaxAge   int
	Speed    int // move down every Speed frames (1 = every frame, 2 = every other, etc.)
	IsSeed   bool // garden mode: seed vs rain
}

// gardenStage drives the lifecycle in garden animation mode.
const (
	gardenStageNone = iota
	gardenStagePlanted
	gardenStageSprout
	gardenStageBud
	gardenStageBloom
	gardenStageWither
	gardenStageEternal
)

type gardenPlot struct {
	stage     int
	moisture  int
	bloomAge  int
	witherAge int
	maxBloom  int // randomized per-bloom duration; computed when entering bloom
	// flowerTint picks a stable accent from gardenFlowerAccentHex for bloom /
	// eternal flower heads; set when a seed first plants the column.
	flowerTint int
}

// GardenTuning controls the pacing and reproductive behavior of garden mode.
// Zero-valued fields are filled in from DefaultGardenTuning() by ResolveGardenTuning.
type GardenTuning struct {
	// SeedSpawnRate is the fraction of new sky drops that fall as seeds (0..1).
	SeedSpawnRate float64

	// RainAbsorbExtraChance is the probability that an extra rain drop in the
	// same frame still counts after the per-column water cap is reached.
	RainAbsorbExtraChance float64

	// Moisture thresholds to advance through plant stages.
	PlantedToSproutMoisture int
	SproutToBudMoisture     int
	BudToBloomMoisture      int

	// SeedMoistureBoost is the moisture added when a seed lands on an existing plot.
	SeedMoistureBoost int

	// Bloom and wither timing (frames). Effective bloom duration is
	// BloomDurationBase + rand.Intn(BloomDurationJitter).
	BloomDurationBase   int
	BloomDurationJitter int
	WitherDuration      int

	// OffspringMin..OffspringMax (inclusive) seeds spawn from each dying plant.
	// OffspringSpread is the half-width of the X jitter window around the parent.
	OffspringMin    int
	OffspringMax    int
	OffspringSpread int
}

// DefaultGardenTuning returns the built-in pacing constants chosen so the
// garden takes its time: most sky pixels are rain, columns advance at most
// once per frame, blooms linger, and dying plants scatter few nearby seeds.
func DefaultGardenTuning() GardenTuning {
	return GardenTuning{
		SeedSpawnRate:           0.055,
		RainAbsorbExtraChance:   0.11,
		PlantedToSproutMoisture: 16,
		SproutToBudMoisture:     22,
		BudToBloomMoisture:      30,
		SeedMoistureBoost:       1,
		BloomDurationBase:       85,
		BloomDurationJitter:     55,
		WitherDuration:          34,
		OffspringMin:            1,
		OffspringMax:            2,
		OffspringSpread:         2,
	}
}

// ResolveGardenTuning fills any zero-valued field in t with the corresponding
// default. Callers (e.g. config plumbing) can pass partial structs.
func ResolveGardenTuning(t GardenTuning) GardenTuning {
	d := DefaultGardenTuning()
	if t.SeedSpawnRate <= 0 {
		t.SeedSpawnRate = d.SeedSpawnRate
	}
	if t.RainAbsorbExtraChance <= 0 {
		t.RainAbsorbExtraChance = d.RainAbsorbExtraChance
	}
	if t.PlantedToSproutMoisture <= 0 {
		t.PlantedToSproutMoisture = d.PlantedToSproutMoisture
	}
	if t.SproutToBudMoisture <= 0 {
		t.SproutToBudMoisture = d.SproutToBudMoisture
	}
	if t.BudToBloomMoisture <= 0 {
		t.BudToBloomMoisture = d.BudToBloomMoisture
	}
	if t.SeedMoistureBoost <= 0 {
		t.SeedMoistureBoost = d.SeedMoistureBoost
	}
	if t.BloomDurationBase <= 0 {
		t.BloomDurationBase = d.BloomDurationBase
	}
	if t.BloomDurationJitter <= 0 {
		t.BloomDurationJitter = d.BloomDurationJitter
	}
	if t.WitherDuration <= 0 {
		t.WitherDuration = d.WitherDuration
	}
	if t.OffspringMin <= 0 {
		t.OffspringMin = d.OffspringMin
	}
	if t.OffspringMax <= 0 {
		t.OffspringMax = d.OffspringMax
	}
	if t.OffspringMax < t.OffspringMin {
		t.OffspringMax = t.OffspringMin
	}
	if t.OffspringSpread <= 0 {
		t.OffspringSpread = d.OffspringSpread
	}
	if t.SeedSpawnRate > 1 {
		t.SeedSpawnRate = 1
	}
	if t.RainAbsorbExtraChance > 1 {
		t.RainAbsorbExtraChance = 1
	}
	return t
}

// gardenTargetStormWallSeconds picks a storm-completion time budget from the
// same TUI presets as config_view (growth pace + seed rate). Used to scale
// tuning so GardenSunny tends to land near these wall times at a reference
// width, adjusted by applyGardenStormWallClockScale for tick + width.
func gardenTargetStormWallSeconds(cfg *config.Config) float64 {
	if cfg == nil {
		return 32
	}
	p := cfg.UI.GardenGrowthPace
	s := cfg.UI.GardenSeedRate
	rareSeed := s > 0 && s < 0.08
	calm := p >= 1.2
	oftenSeed := s > 0.12
	fastPace := p > 0 && p < 0.9

	switch {
	case rareSeed && calm:
		return 180
	case rareSeed:
		return 150
	case calm:
		return 60
	case oftenSeed && fastPace:
		return 26
	case oftenSeed || fastPace:
		return 36
	default:
		return 32
	}
}

// applyGardenStormWallClockScale stretches resolved garden pacing so the
// storm phase (until ~80% visual fill / GardenSunny) tends toward
// gardenTargetStormWallSeconds, scaled by rain tick (ms/frame) and sqrt(width)
// so wider terminals get proportionally more frames for the same wall time.
func applyGardenStormWallClockScale(t *GardenTuning, cfg *config.Config, rainTickMS, gardenWidth int) {
	if t == nil {
		return
	}
	tick := rainTickMS
	if tick <= 0 {
		tick = config.DefaultUIRainTickMS
	}
	sec := gardenTargetStormWallSeconds(cfg)
	w := float64(gardenWidth)
	if w < 12 {
		w = 12
	}
	const refW = 56.0
	widthNorm := math.Sqrt(w / refW)
	targetFrames := sec * (1000.0 / float64(tick)) * widthNorm

	// Empirical baseline: pre–wall-clock tuning tended to finish the storm in
	// roughly this many frames at default moisture + seed rates. Ratio maps
	// desired wall time into multipliers on thresholds and spawn rate.
	const refFrames = 66.0
	scale := targetFrames / refFrames
	if scale < 1.0 {
		scale = 1.0
	}
	// Slightly extra stretch on the path to first bloom; sky seeds are capped
	// separately, so moisture still needs headroom to feel gradual.
	const moistureBloomStretch = 1.12
	scaleMoist := scale * moistureBloomStretch

	scaleInt := func(v int, m float64) int {
		x := int(float64(v)*m + 0.5)
		if x < 1 {
			return 1
		}
		return x
	}
	t.PlantedToSproutMoisture = scaleInt(t.PlantedToSproutMoisture, scaleMoist)
	t.SproutToBudMoisture = scaleInt(t.SproutToBudMoisture, scaleMoist)
	t.BudToBloomMoisture = scaleInt(t.BudToBloomMoisture, scaleMoist)
	t.BloomDurationBase = scaleInt(t.BloomDurationBase, scale)
	t.BloomDurationJitter = scaleInt(t.BloomDurationJitter, scale)
	t.WitherDuration = scaleInt(t.WitherDuration, scale)

	if t.SeedMoistureBoost > 0 {
		b := int(float64(t.SeedMoistureBoost)/math.Sqrt(scale) + 0.5)
		if b < 1 {
			b = 1
		}
		t.SeedMoistureBoost = b
	}

	t.SeedSpawnRate /= scale
	if t.SeedSpawnRate < 0.008 {
		t.SeedSpawnRate = 0.008
	}
	if t.SeedSpawnRate > 0.45 {
		t.SeedSpawnRate = 0.45
	}

	t.RainAbsorbExtraChance /= scale*0.85 + 0.15
	if t.RainAbsorbExtraChance < 0.02 {
		t.RainAbsorbExtraChance = 0.02
	}
	if t.RainAbsorbExtraChance > 0.55 {
		t.RainAbsorbExtraChance = 0.55
	}
}

// flowerCell tracks accumulated rainfall at a column for the advanced animation
type flowerCell struct {
	drops int // accumulated drop count at this column
}

// RainBackground manages the animated rain background
type RainBackground struct {
	Width       int
	Height      int
	Drops       []RainDrop
	Frame       int
	Mode        string // "basic" or "advanced"
	Flowers     []flowerCell
	CloudRow    []string // pre-rendered cloud chars per column
	GardenPlots []gardenPlot
	GardenSunny bool         // garden mode: rain finished, sky cleared
	Garden      GardenTuning // pacing knobs (always resolved to non-zero values)
}

// NewRainBackground creates a new rain background
func NewRainBackground(width, height int, mode string) *RainBackground {
	rb := &RainBackground{
		Width:  width,
		Height: height,
		Drops:  make([]RainDrop, 0),
		Frame:  0,
		Mode:   mode,
		Garden: DefaultGardenTuning(),
	}
	if width > 0 {
		rb.Flowers = make([]flowerCell, width)
		rb.CloudRow = rb.buildCloudRow()
	}
	if width > 0 && mode == config.UIRainAnimationGarden {
		rb.GardenPlots = make([]gardenPlot, width)
	}
	rb.Reset()
	return rb
}

// SetGardenTuning replaces the active garden tuning. Zero-valued fields in t
// are resolved to defaults; callers can pass partial structs.
func (rb *RainBackground) SetGardenTuning(t GardenTuning) {
	rb.Garden = ResolveGardenTuning(t)
}

func (rb *RainBackground) buildCloudRow() []string {
	row := make([]string, rb.Width)
	for x := 0; x < rb.Width; x++ {
		if rand.Float64() < 0.55 {
			row[x] = cloudChars[rand.Intn(len(cloudChars))]
		} else {
			row[x] = " "
		}
	}
	return row
}

// Reset reinitializes all drops
func (rb *RainBackground) Reset() {
	rb.Drops = make([]RainDrop, 0)
	rb.GardenSunny = false
	if rb.Mode == config.UIRainAnimationGarden && rb.Width > 0 {
		rb.GardenPlots = make([]gardenPlot, rb.Width)
	}
	startY := 0
	if rb.Mode == config.UIRainAnimationAdvanced || rb.Mode == config.UIRainAnimationGarden {
		startY = 1 // leave top row for clouds / sky
	}
	targetCount := rb.Width * 2
	if rb.Mode == config.UIRainAnimationGarden {
		rb.spawnGardenMaintainingDrops(startY, targetCount)
	} else {
		for i := 0; i < targetCount; i++ {
			rb.spawnDrop(startY)
		}
	}
}

// spawnDrop creates a new raindrop at or near the top of the field
func (rb *RainBackground) spawnDrop(minY int) {
	if rb.Width <= 0 || rb.Height <= 0 {
		return
	}
	startY := minY
	if rb.Height > 2 {
		startY = minY + rand.Intn(rb.Height/3)
	}
	speed := 1 + rand.Intn(2) // 1 or 2 frames per step
	char := rainDropChars[rand.Intn(len(rainDropChars))]
	isSeed := false
	if rb.Mode == config.UIRainAnimationMatrix {
		char = matrixGlyphPool[rand.Intn(len(matrixGlyphPool))]
	}
	drop := RainDrop{
		X:        rand.Intn(rb.Width),
		Y:        startY,
		Char:     char,
		ColorIdx: 0,
		Age:      0,
		MaxAge:   rb.Height + rand.Intn(6),
		Speed:    speed,
		IsSeed:   isSeed,
	}
	rb.Drops = append(rb.Drops, drop)
}

// spawnDropGarden appends one garden storm sky drop (rain or seed).
func (rb *RainBackground) spawnDropGarden(minY int, isSeed bool) {
	if rb.Width <= 0 || rb.Height <= 0 {
		return
	}
	startY := minY
	if rb.Height > 2 {
		startY = minY + rand.Intn(rb.Height/3)
	}
	speed := 1 + rand.Intn(2)
	char := rainDropChars[rand.Intn(len(rainDropChars))]
	if isSeed {
		char = "∘"
		speed = 2 + rand.Intn(2) // seeds fall a little slower
	}
	rb.Drops = append(rb.Drops, RainDrop{
		X:        rand.Intn(rb.Width),
		Y:        startY,
		Char:     char,
		ColorIdx: 0,
		Age:      0,
		MaxAge:   rb.Height + rand.Intn(6),
		Speed:    speed,
		IsSeed:   isSeed,
	})
}

func gardenBinomial(n int, p float64) int {
	if n <= 0 || p <= 0 {
		return 0
	}
	if p >= 1 {
		return n
	}
	c := 0
	for i := 0; i < n; i++ {
		if rand.Float64() < p {
			c++
		}
	}
	return c
}

// gardenRandSeedMask picks exactly k true entries among n (uniform subset).
func gardenRandSeedMask(n, k int) []bool {
	m := make([]bool, n)
	if k <= 0 {
		return m
	}
	if k >= n {
		for i := range m {
			m[i] = true
		}
		return m
	}
	perm := rand.Perm(n)
	for i := 0; i < k; i++ {
		m[perm[i]] = true
	}
	return m
}

func (rb *RainBackground) gardenSeedsInFlight() int {
	n := 0
	for i := range rb.Drops {
		if rb.Drops[i].IsSeed {
			n++
		}
	}
	return n
}

// gardenSeedThrottleRelief is in [0,1]: low when plots are empty or early
// growth, high when many columns are in full bloom (or eternal). It relaxes
// the global flying-seed ceiling so young plants see few sky seeds, and mature
// meadows can carry more falling seeds.
func (rb *RainBackground) gardenSeedThrottleRelief() float64 {
	if rb.GardenPlots == nil || len(rb.GardenPlots) == 0 {
		return 0
	}
	var sum float64
	for i := range rb.GardenPlots {
		switch rb.GardenPlots[i].stage {
		case gardenStageNone:
			sum += 0
		case gardenStagePlanted:
			sum += 0.1
		case gardenStageSprout:
			sum += 0.28
		case gardenStageBud:
			sum += 0.5
		case gardenStageBloom:
			sum += 1.0
		case gardenStageWither:
			sum += 0.72
		case gardenStageEternal:
			sum += 1.0
		default:
			sum += 0
		}
	}
	return sum / float64(len(rb.GardenPlots))
}

// gardenMaxFlyingSkySeeds returns how many seed drops may exist at once for
// this garden maturity (relief). Early stages get a low ceiling; full bloom
// raises it roughly linearly between lo and hi.
func (rb *RainBackground) gardenMaxFlyingSkySeeds(relief float64) int {
	if relief < 0 {
		relief = 0
	}
	if relief > 1 {
		relief = 1
	}
	rate := rb.Garden.SeedSpawnRate
	if rate <= 0 {
		rate = DefaultGardenTuning().SeedSpawnRate
	}
	w := float64(rb.Width)
	hi := int(0.5 + w*rate*0.55 + 4)
	if hi < 4 {
		hi = 4
	}
	lo := max(1, hi/5)
	cap := int(0.5 + float64(lo)+(float64(hi-lo))*relief)
	if cap < 1 {
		cap = 1
	}
	if cap > hi {
		cap = hi
	}
	return cap
}

// gardenSkySeedsForBatch returns how many of 'need' new sky drops should be
// seeds. Refill often adds many drops in one frame; independent Bernoulli rolls
// would otherwise flood wide terminals with seeds. The count is also clamped
// by gardenMaxFlyingSkySeeds minus seeds already in flight, with the ceiling
// rising as plots reach bloom (gardenSeedThrottleRelief).
func (rb *RainBackground) gardenSkySeedsForBatch(need int) int {
	if need <= 0 || rb.GardenSunny {
		return 0
	}
	rate := rb.Garden.SeedSpawnRate
	if rate <= 0 {
		return 0
	}
	maxSeeds := int(0.5 + float64(rb.Width)*rate*0.20)
	if maxSeeds < 1 {
		maxSeeds = 1
	}
	if maxSeeds > need {
		maxSeeds = need
	}
	raw := gardenBinomial(need, rate)
	k := raw
	if k > maxSeeds {
		k = maxSeeds
	}
	relief := rb.gardenSeedThrottleRelief()
	flying := rb.gardenSeedsInFlight()
	slotBudget := rb.gardenMaxFlyingSkySeeds(relief) - flying
	if slotBudget <= 0 {
		return 0
	}
	if k > slotBudget {
		k = slotBudget
	}
	return k
}

func (rb *RainBackground) spawnGardenMaintainingDrops(minY, targetCount int) {
	if rb.Width <= 0 || rb.Height <= 0 {
		return
	}
	for len(rb.Drops) < targetCount {
		need := targetCount - len(rb.Drops)
		k := rb.gardenSkySeedsForBatch(need)
		mask := gardenRandSeedMask(need, k)
		for i := 0; i < need; i++ {
			rb.spawnDropGarden(minY, mask[i])
		}
	}
}

// Update advances the animation by one frame
func (rb *RainBackground) Update() {
	rb.Frame++

	minY := 0
	maxDropY := rb.Height - 1
	if rb.Mode == config.UIRainAnimationAdvanced || rb.Mode == config.UIRainAnimationGarden {
		minY = 1
		maxDropY = rb.Height - 2 // leave bottom row for plants / flowers
	}

	if rb.Mode == config.UIRainAnimationGarden && rb.GardenSunny {
		rb.gardenAdvancePlotsSunny()
		return
	}

	// Per-frame water cap: each garden column accepts at most one rain hit
	// per Update, with a small chance of accepting a second so dense rain
	// still reads as "soaking" without machine-gunning growth thresholds.
	var gardenWatered []bool
	if rb.Mode == config.UIRainAnimationGarden && !rb.GardenSunny && rb.Width > 0 {
		gardenWatered = make([]bool, rb.Width)
	}

	for i := range rb.Drops {
		p := &rb.Drops[i]
		p.Age++

		// Move down based on speed
		if rb.Frame%p.Speed == 0 {
			p.Y++
		}

		// Slight horizontal drift
		driftChance := 0.15 + 0.05*math.Sin(float64(rb.Frame)*0.07)
		if rand.Float64() < driftChance {
			p.X += rand.Intn(3) - 1
		}
		if p.X < 0 {
			p.X = 0
		}
		if p.X >= rb.Width {
			p.X = rb.Width - 1
		}

		// Color gradient: top (dark) → bottom (bright)
		progress := float64(p.Y-minY) / float64(rb.Height-minY)
		if progress < 0 {
			progress = 0
		}
		paletteLen := len(rainPaletteForMode(rb.Mode))
		if paletteLen == 0 {
			paletteLen = 1
		}
		p.ColorIdx = int(progress * float64(paletteLen-1))
		if p.ColorIdx >= paletteLen {
			p.ColorIdx = paletteLen - 1
		}

		// In advanced mode, accumulate flowers when a drop reaches the bottom
		if rb.Mode == config.UIRainAnimationAdvanced && p.Y >= maxDropY && rb.Flowers != nil && p.X < len(rb.Flowers) {
			rb.Flowers[p.X].drops++
		}

		if rb.Mode == config.UIRainAnimationGarden && p.Y >= maxDropY && rb.GardenPlots != nil && p.X >= 0 && p.X < len(rb.GardenPlots) {
			g := &rb.GardenPlots[p.X]
			if p.IsSeed {
				if g.stage == gardenStageNone {
					g.stage = gardenStagePlanted
					g.moisture = 0
					g.bloomAge = 0
					g.witherAge = 0
					if len(gardenFlowerAccentHex) > 0 {
						g.flowerTint = rand.Intn(len(gardenFlowerAccentHex))
					}
				} else {
					g.moisture += rb.Garden.SeedMoistureBoost
				}
			} else if gardenWatered != nil {
				if !gardenWatered[p.X] {
					rb.gardenWaterPlot(g)
					gardenWatered[p.X] = true
				} else if rb.Garden.RainAbsorbExtraChance > 0 && rand.Float64() < rb.Garden.RainAbsorbExtraChance {
					rb.gardenWaterPlot(g)
				}
			}
		}
	}

	// Remove dead drops (off screen or expired)
	alive := rb.Drops[:0]
	for _, p := range rb.Drops {
		if p.Y < rb.Height && p.Age < p.MaxAge {
			alive = append(alive, p)
		}
	}
	rb.Drops = alive

	// Spawn new drops to maintain count
	if rb.Width > 0 && rb.Height > 0 && !(rb.Mode == config.UIRainAnimationGarden && rb.GardenSunny) {
		targetCount := rb.Width * 2
		if rb.Mode == config.UIRainAnimationGarden {
			rb.spawnGardenMaintainingDrops(minY, targetCount)
		} else {
			for len(rb.Drops) < targetCount {
				rb.spawnDrop(minY)
			}
		}
	}

	// Periodically refresh cloud row in advanced / garden (storm) mode
	if (rb.Mode == config.UIRainAnimationAdvanced || rb.Mode == config.UIRainAnimationGarden) && rb.Frame%30 == 0 && rb.Width > 0 {
		rb.CloudRow = rb.buildCloudRow()
	}

	if rb.Mode == config.UIRainAnimationGarden {
		rb.gardenAdvancePlots()
		rb.gardenMaybeFinishStorm()
	}
}

// Render returns the rain background as a string
func (rb *RainBackground) Render() string {
	if rb.Width <= 0 || rb.Height <= 0 {
		return ""
	}

	cells := make([]string, rb.Width*rb.Height)
	for i := range cells {
		cells[i] = " "
	}

	styles := rainColorStylesForMode(rb.Mode)

	// Place raindrops
	for _, p := range rb.Drops {
		if p.Y >= 0 && p.Y < rb.Height && p.X >= 0 && p.X < rb.Width {
			if len(styles) == 0 {
				continue
			}
			safeIdx := p.ColorIdx % len(styles)
			if safeIdx < 0 {
				safeIdx += len(styles)
			}
			cellIdx := p.Y*rb.Width + p.X
			cells[cellIdx] = styles[safeIdx].Render(p.Char)
		}
	}

	if rb.Mode == config.UIRainAnimationGarden {
		rb.paintGardenOverlays(cells)
	} else if rb.Mode == config.UIRainAnimationAdvanced {
		// Top row: clouds
		if len(rb.CloudRow) >= rb.Width {
			cloudStyle := lipgloss.NewStyle().Foreground(activeProfile().cloudColor)
			for x := 0; x < rb.Width; x++ {
				cells[x] = cloudStyle.Render(rb.CloudRow[x])
			}
		}
		// Bottom row: flowers
		if rb.Height > 1 && rb.Flowers != nil {
			bottomY := rb.Height - 1
			for x := 0; x < rb.Width && x < len(rb.Flowers); x++ {
				stage := rb.flowerStage(x)
				if stage >= 0 {
					flowerStyle := lipgloss.NewStyle().Foreground(activeProfile().flowerColor)
					cells[bottomY*rb.Width+x] = flowerStyle.Render(flowerStages[stage])
				}
			}
		}
	}

	var result strings.Builder
	result.Grow(rb.Width*rb.Height*2 + rb.Height)
	for y := 0; y < rb.Height; y++ {
		for x := 0; x < rb.Width; x++ {
			result.WriteString(cells[y*rb.Width+x])
		}
		if y < rb.Height-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

func (rb *RainBackground) gardenMaxAirRows() int {
	if rb.Height <= 1 {
		return 0
	}
	return rb.Height - 1
}

func gardenColumnVisualRows(stage, maxRows int) int {
	if maxRows <= 0 {
		return 0
	}
	switch stage {
	case gardenStageNone:
		return 0
	case gardenStagePlanted:
		return min(1, maxRows)
	case gardenStageSprout:
		return min(2, maxRows)
	case gardenStageBud:
		return min(3, maxRows)
	case gardenStageBloom, gardenStageWither, gardenStageEternal:
		return maxRows
	default:
		return 0
	}
}

func gardenGlyph(stage, layerFromBottom, visualH int) string {
	switch stage {
	case gardenStagePlanted:
		return "∘"
	case gardenStageSprout:
		if layerFromBottom == 0 {
			return "·"
		}
		return "╷"
	case gardenStageBud:
		switch layerFromBottom {
		case 0:
			return "♦"
		case 1:
			return "┊"
		default:
			return "╷"
		}
	case gardenStageBloom, gardenStageEternal:
		if visualH <= 1 {
			return "✿"
		}
		if layerFromBottom == 0 {
			return "│"
		}
		if layerFromBottom == visualH-1 {
			return "❀"
		}
		if layerFromBottom == visualH-2 {
			return "✿"
		}
		return "⌣"
	case gardenStageWither:
		if layerFromBottom == 0 {
			return "˙"
		}
		if layerFromBottom == visualH-1 {
			return "·"
		}
		return "⁘"
	default:
		return " "
	}
}

func (rb *RainBackground) gardenWaterPlot(g *gardenPlot) {
	if g.stage == gardenStageNone || g.stage == gardenStageEternal {
		return
	}
	if g.stage == gardenStageWither {
		return
	}
	g.moisture++
	switch g.stage {
	case gardenStagePlanted:
		if g.moisture >= rb.Garden.PlantedToSproutMoisture {
			g.stage = gardenStageSprout
			g.moisture = 0
		}
	case gardenStageSprout:
		if g.moisture >= rb.Garden.SproutToBudMoisture {
			g.stage = gardenStageBud
			g.moisture = 0
		}
	case gardenStageBud:
		if g.moisture >= rb.Garden.BudToBloomMoisture {
			g.stage = gardenStageBloom
			g.moisture = 0
			g.bloomAge = 0
			g.maxBloom = rb.gardenRollBloomDuration()
		}
	}
}

// gardenRollBloomDuration returns a randomized bloom lifetime in frames.
func (rb *RainBackground) gardenRollBloomDuration() int {
	base := rb.Garden.BloomDurationBase
	jit := rb.Garden.BloomDurationJitter
	if jit > 0 {
		base += rand.Intn(jit)
	}
	if base < 1 {
		base = 1
	}
	return base
}

func (rb *RainBackground) gardenAdvancePlots() {
	if rb.GardenPlots == nil {
		return
	}
	for i := range rb.GardenPlots {
		g := &rb.GardenPlots[i]
		switch g.stage {
		case gardenStageBloom:
			g.bloomAge++
			maxBloom := g.maxBloom
			if maxBloom <= 0 {
				maxBloom = rb.gardenRollBloomDuration()
				g.maxBloom = maxBloom
			}
			if g.bloomAge >= maxBloom {
				g.stage = gardenStageWither
				g.witherAge = 0
			}
		case gardenStageWither:
			g.witherAge++
			if g.witherAge >= rb.Garden.WitherDuration {
				g.stage = gardenStageNone
				g.moisture = 0
				g.bloomAge = 0
				g.witherAge = 0
				g.maxBloom = 0
				g.flowerTint = 0
				rb.spawnGardenSeedsBurst(i)
			}
		}
	}
}

func (rb *RainBackground) gardenAdvancePlotsSunny() {}

func (rb *RainBackground) gardenFillPortion() (num, denom int) {
	air := rb.gardenMaxAirRows()
	if air <= 0 || rb.Width <= 0 {
		return 0, 1
	}
	denom = rb.Width * air
	if rb.GardenPlots == nil {
		return 0, denom
	}
	for x := 0; x < rb.Width && x < len(rb.GardenPlots); x++ {
		num += gardenColumnVisualRows(rb.GardenPlots[x].stage, air)
	}
	return num, denom
}

func (rb *RainBackground) gardenMaybeFinishStorm() {
	if rb.GardenSunny || rb.GardenPlots == nil {
		return
	}
	num, denom := rb.gardenFillPortion()
	if denom <= 0 {
		return
	}
	if num*100 >= denom*80 {
		rb.GardenSunny = true
		rb.Drops = nil
		for i := range rb.GardenPlots {
			if rb.GardenPlots[i].stage != gardenStageNone {
				rb.GardenPlots[i].stage = gardenStageEternal
				rb.GardenPlots[i].moisture = 0
				rb.GardenPlots[i].bloomAge = 0
				rb.GardenPlots[i].witherAge = 0
			}
		}
	}
}

func (rb *RainBackground) spawnGardenSeed(x int) {
	if rb.Width <= 0 || rb.Height <= 0 || rb.GardenSunny {
		return
	}
	if x < 0 || x >= rb.Width {
		return
	}
	startY := 1
	if rb.Height > 3 {
		// Stagger so multiple offspring don't appear in lockstep.
		startY = 1 + rand.Intn(2)
	}
	speed := 2 + rand.Intn(2)
	rb.Drops = append(rb.Drops, RainDrop{
		X:        x,
		Y:        startY,
		Char:     "∘",
		ColorIdx: 0,
		Age:      0,
		MaxAge:   rb.Height + 10,
		Speed:    speed,
		IsSeed:   true,
	})
}

// spawnGardenSeedsBurst scatters OffspringMin..OffspringMax seeds around
// originX so a dying plant repopulates a small neighborhood instead of
// directly replacing itself. Offsets are clamped to [0, width-1].
func (rb *RainBackground) spawnGardenSeedsBurst(originX int) {
	if rb.Width <= 0 || rb.Height <= 0 || rb.GardenSunny {
		return
	}
	minN := rb.Garden.OffspringMin
	maxN := rb.Garden.OffspringMax
	if minN < 1 {
		minN = 1
	}
	if maxN < minN {
		maxN = minN
	}
	n := minN
	if maxN > minN {
		n += rand.Intn(maxN - minN + 1)
	}
	spread := rb.Garden.OffspringSpread
	if spread < 0 {
		spread = 0
	}
	for k := 0; k < n; k++ {
		x := originX
		if spread > 0 {
			x += rand.Intn(2*spread+1) - spread
		}
		if x < 0 {
			x = 0
		}
		if x >= rb.Width {
			x = rb.Width - 1
		}
		rb.spawnGardenSeed(x)
	}
}

func (rb *RainBackground) paintGardenOverlays(cells []string) {
	if rb.Width <= 0 || rb.Height <= 0 || rb.GardenPlots == nil {
		return
	}
	air := rb.gardenMaxAirRows()
	skyBlue := lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB"))
	skyLight := lipgloss.NewStyle().Foreground(lipgloss.Color("#B3E5FC"))
	sunSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFEB3B")).Bold(true)
	cloudStyle := lipgloss.NewStyle().Foreground(activeProfile().cloudColor)

	if rb.GardenSunny {
		sunX := rb.Width / 2
		for x := 0; x < rb.Width; x++ {
			if x == sunX {
				cells[x] = sunSt.Render("☀")
			} else if (x+rb.Frame)%3 == 0 {
				cells[x] = skyLight.Render("░")
			} else {
				cells[x] = skyBlue.Render("░")
			}
		}
	} else if len(rb.CloudRow) >= rb.Width {
		for x := 0; x < rb.Width; x++ {
			cells[x] = cloudStyle.Render(rb.CloudRow[x])
		}
	}

	stemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1B5E20"))
	leafStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#43A047"))
	witherStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6D4C41"))

	for x := 0; x < rb.Width && x < len(rb.GardenPlots); x++ {
		g := rb.GardenPlots[x]
		h := gardenColumnVisualRows(g.stage, air)
		for k := 0; k < h; k++ {
			y := rb.Height - 1 - k
			ch := gardenGlyph(g.stage, k, h)
			var st lipgloss.Style
			switch g.stage {
			case gardenStageWither:
				st = witherStyle
			case gardenStageEternal:
				switch {
				case k == 0:
					st = stemStyle
				case k == h-1:
					st = gardenFlowerStyle(g.flowerTint).Bold(true)
				default:
					st = leafStyle
				}
			default:
				switch {
				case g.stage == gardenStagePlanted && k == 0:
					st = lipgloss.NewStyle().Foreground(lipgloss.Color("#5D4037"))
				case k == 0 && g.stage >= gardenStageBud:
					st = stemStyle
				case k >= h-1 && g.stage >= gardenStageBud:
					st = gardenFlowerStyle(g.flowerTint)
				default:
					st = leafStyle
				}
			}
			idx := y*rb.Width + x
			if idx >= 0 && idx < len(cells) {
				cells[idx] = st.Render(ch)
			}
		}
	}
}

// flowerStage returns the growth stage index (0-3) or -1 if no drops yet.
func (rb *RainBackground) flowerStage(x int) int {
	if rb.Flowers == nil || x >= len(rb.Flowers) {
		return -1
	}
	drops := rb.Flowers[x].drops
	switch {
	case drops == 0:
		return -1
	case drops < 3:
		return 0
	case drops < 8:
		return 1
	case drops < 15:
		return 2
	default:
		return 3
	}
}

// RenderRainWave renders a storm-cloud wave strip for the top of the TUI.
// In basic mode it's a wave of drop chars; in advanced mode it shows a cloud line.
// For garden mode, gardenSunny selects a clear sky strip after the storm ends.
func RenderRainWave(width, frame int, mode string, gardenSunny bool) string {
	var result strings.Builder
	styles := rainColorStylesForMode(mode)
	if len(styles) == 0 {
		return strings.Repeat("~", width)
	}

	if mode == config.UIRainAnimationGarden && gardenSunny {
		skyBlue := lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB"))
		skyLight := lipgloss.NewStyle().Foreground(lipgloss.Color("#B3E5FC"))
		sunSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFEB3B")).Bold(true)
		sunX := width / 2
		for x := 0; x < width; x++ {
			if x == sunX {
				result.WriteString(sunSt.Render("☀"))
			} else if (x+frame)%3 == 0 {
				result.WriteString(skyLight.Render("░"))
			} else {
				result.WriteString(skyBlue.Render("░"))
			}
		}
		return result.String()
	}

	if mode == config.UIRainAnimationAdvanced || mode == config.UIRainAnimationGarden {
		// Render a cloud band across the full width
		cloudStyle := lipgloss.NewStyle().Foreground(activeProfile().cloudColor)
		for x := 0; x < width; x++ {
			phase := float64(frame)*0.04 + float64(x)*0.18
			v := math.Sin(phase)
			var ch string
			if v > 0.5 {
				ch = "▓"
			} else if v > 0 {
				ch = "▒"
			} else if v > -0.5 {
				ch = "░"
			} else {
				ch = "☁"
			}
			result.WriteString(cloudStyle.Render(ch))
		}
		return result.String()
	}

	if mode == config.UIRainAnimationMatrix {
		for x := 0; x < width; x++ {
			phase := float64(frame)*0.075 + float64(x)*0.24
			y := 0.75*math.Sin(float64(x)*0.24+phase) + 0.25*math.Sin(float64(x)*0.11+phase*0.6)
			char := matrixGlyphPool[(x+frame+int(y*7))%len(matrixGlyphPool)]
			colorIdx := int(float64(x) / float64(width) * float64(len(styles)-1))
			if colorIdx >= len(styles) {
				colorIdx = len(styles) - 1
			}
			result.WriteString(styles[colorIdx].Render(char))
		}
		return result.String()
	}

	// Basic mode: a sine-wave of drop characters at varying depths
	for x := 0; x < width; x++ {
		phase := float64(frame) * 0.075
		y := 0.75*math.Sin(float64(x)*0.24+phase) + 0.25*math.Sin(float64(x)*0.11+phase*0.6)

		var char string
		if y > 0.65 {
			char = "│"
		} else if y > 0.25 {
			char = "╷"
		} else if y > 0 {
			char = "·"
		} else if y > -0.25 {
			char = "˙"
		} else if y > -0.65 {
			char = "⁚"
		} else {
			char = "⋮"
		}

		colorIdx := int(float64(x) / float64(width) * float64(len(styles)-1))
		if colorIdx >= len(styles) {
			colorIdx = len(styles) - 1
		}
		result.WriteString(styles[colorIdx].Render(char))
	}
	return result.String()
}

func rainPaletteForMode(mode string) []lipgloss.Color {
	if mode == config.UIRainAnimationMatrix {
		return matrixRainColors
	}
	if mode == config.UIRainAnimationGarden {
		return gardenRainColors
	}
	if len(activeRainColors) == 0 {
		return []lipgloss.Color{lipgloss.Color("#4488CC")}
	}
	return activeRainColors
}

func rainColorStylesForMode(mode string) []lipgloss.Style {
	colors := rainPaletteForMode(mode)
	if len(colors) == 0 {
		return []lipgloss.Style{
			lipgloss.NewStyle().Foreground(lipgloss.Color("#4488CC")),
		}
	}
	styles := make([]lipgloss.Style, len(colors))
	for i, color := range colors {
		styles[i] = lipgloss.NewStyle().Foreground(color)
	}
	return styles
}

// matrixRainColors is a green terminal-rain palette (dark → bright).
var matrixRainColors = []lipgloss.Color{
	lipgloss.Color("#001A00"),
	lipgloss.Color("#003B00"),
	lipgloss.Color("#006400"),
	lipgloss.Color("#008F11"),
	lipgloss.Color("#00AA22"),
	lipgloss.Color("#22CC44"),
	lipgloss.Color("#55EE77"),
	lipgloss.Color("#CCFFCC"),
}

// gardenRainColors is a soft rain palette for the growing season.
var gardenRainColors = []lipgloss.Color{
	lipgloss.Color("#1A237E"),
	lipgloss.Color("#283593"),
	lipgloss.Color("#303F9F"),
	lipgloss.Color("#3949AB"),
	lipgloss.Color("#5C6BC0"),
	lipgloss.Color("#7986CB"),
	lipgloss.Color("#9FA8DA"),
	lipgloss.Color("#C5CAE9"),
}
