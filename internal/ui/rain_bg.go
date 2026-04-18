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
}

// flowerCell tracks accumulated rainfall at a column for the advanced animation
type flowerCell struct {
	drops int // accumulated drop count at this column
}

// RainBackground manages the animated rain background
type RainBackground struct {
	Width    int
	Height   int
	Drops    []RainDrop
	Frame    int
	Mode     string // "basic" or "advanced"
	Flowers  []flowerCell
	CloudRow []string // pre-rendered cloud chars per column
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
	startY := 0
	if rb.Mode == config.UIRainAnimationAdvanced {
		startY = 1 // leave top row for clouds
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
	if rb.Mode == config.UIRainAnimationMatrix {
		char = matrixGlyphPool[rand.Intn(len(matrixGlyphPool))]
		// Rarely swap in a subliminal ASCII cell (single-column safe).
		if rand.Float64() < 0.04 {
			if c, ok := matrixMarqueeChar(rand.Intn(rb.Width), rb.Frame, rb.Width); ok {
				char = c
			}
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
	}
	rb.Drops = append(rb.Drops, drop)
}

// Update advances the animation by one frame
func (rb *RainBackground) Update() {
	rb.Frame++

	minY := 0
	maxDropY := rb.Height - 1
	if rb.Mode == config.UIRainAnimationAdvanced {
		minY = 1
		maxDropY = rb.Height - 2 // leave bottom row for flowers
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

		if rb.Mode == config.UIRainAnimationMatrix && rand.Float64() < 0.02 {
			if c, ok := matrixMarqueeChar(p.X, rb.Frame, rb.Width); ok {
				p.Char = c
			}
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
	if rb.Width > 0 && rb.Height > 0 {
		targetCount := rb.Width * 2
		for len(rb.Drops) < targetCount {
			rb.spawnDrop(minY)
		}
	}

	// Periodically refresh cloud row in advanced mode
	if rb.Mode == config.UIRainAnimationAdvanced && rb.Frame%30 == 0 && rb.Width > 0 {
		rb.CloudRow = rb.buildCloudRow()
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

	if rb.Mode == config.UIRainAnimationAdvanced {
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

	if rb.Mode == config.UIRainAnimationMatrix {
		subY := matrixSubliminalBackgroundRow(rb.Height)
		if subY >= 0 && subY < rb.Height && len(styles) > 0 {
			dim := lipgloss.NewStyle().
				Foreground(matrixRainColors[2]).
				Faint(true)
			mid := len(styles) / 2
			if mid < 0 {
				mid = 0
			}
			for x := 0; x < rb.Width; x++ {
				if c, ok := matrixMarqueeCharBackground(x, rb.Frame, rb.Width, rb.Height); ok {
					cells[subY*rb.Width+x] = dim.Render(c)
				} else if rand.Float64() < 0.012 {
					cells[subY*rb.Width+x] = styles[mid].Faint(true).Render(matrixGlyphPool[(x+rb.Frame)%len(matrixGlyphPool)])
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
func RenderRainWave(width, frame int, mode string) string {
	var result strings.Builder
	styles := rainColorStylesForMode(mode)
	if len(styles) == 0 {
		return strings.Repeat("~", width)
	}

	if mode == config.UIRainAnimationAdvanced {
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
			style := styles[colorIdx]
			if c, ok := matrixWaveMaybeSubliminal(x, frame, width); ok {
				char = c
				style = style.Faint(true)
			} else if width > 0 && x == width/2 && frame%47 == 0 {
				if c, ok := matrixVerticalSubliminalChar(frame / 47); ok {
					char = c
					style = style.Faint(true)
				}
			}
			result.WriteString(style.Render(char))
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
