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
	GardenSunny bool // garden mode: rain finished, sky cleared
}

// NewRainBackground creates a new rain background
func NewRainBackground(width, height int, mode string) *RainBackground {
	rb := &RainBackground{
		Width:  width,
		Height: height,
		Drops:  make([]RainDrop, 0),
		Frame:  0,
		Mode:   mode,
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
	for i := 0; i < targetCount; i++ {
		rb.spawnDrop(startY)
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
	if rb.Mode == config.UIRainAnimationGarden && !rb.GardenSunny {
		if rand.Float64() < 0.28 {
			isSeed = true
			char = "∘"
			speed = 2 + rand.Intn(2) // seeds fall a little slower
		}
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
				} else {
					g.moisture += 2
				}
			} else {
				rb.gardenWaterPlot(g)
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
		for len(rb.Drops) < targetCount {
			rb.spawnDrop(minY)
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
		if g.moisture >= 4 {
			g.stage = gardenStageSprout
			g.moisture = 0
		}
	case gardenStageSprout:
		if g.moisture >= 6 {
			g.stage = gardenStageBud
			g.moisture = 0
		}
	case gardenStageBud:
		if g.moisture >= 8 {
			g.stage = gardenStageBloom
			g.moisture = 0
			g.bloomAge = 0
		}
	}
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
			maxBloom := 36 + (i % 19)
			if g.bloomAge >= maxBloom {
				g.stage = gardenStageWither
				g.witherAge = 0
			}
		case gardenStageWither:
			g.witherAge++
			if g.witherAge >= 18 {
				g.stage = gardenStageNone
				g.moisture = 0
				g.bloomAge = 0
				g.witherAge = 0
				rb.spawnGardenSeed(i)
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
	rb.Drops = append(rb.Drops, RainDrop{
		X:        x,
		Y:        startY,
		Char:     "∘",
		ColorIdx: 0,
		Age:      0,
		MaxAge:   rb.Height + 10,
		Speed:    2,
		IsSeed:   true,
	})
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
	flowerStyle := lipgloss.NewStyle().Foreground(activeProfile().flowerColor)
	witherStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6D4C41"))
	eternalFlower := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8BBD0")).Bold(true)

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
					st = eternalFlower
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
					st = flowerStyle
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
