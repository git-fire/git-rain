package ui

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

var rainDropChars = [...]string{"│", "╵", "·", "˙", "╷", "⁚", "⋮"}

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
	drop := RainDrop{
		X:        rand.Intn(rb.Width),
		Y:        startY,
		Char:     rainDropChars[rand.Intn(len(rainDropChars))],
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

		// Color gradient: top (dark) → bottom (bright/white)
		progress := float64(p.Y-minY) / float64(rb.Height-minY)
		if progress < 0 {
			progress = 0
		}
		paletteLen := len(activeRainColors)
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

	styles := rainColorStyles()

	// Place raindrops
	for _, p := range rb.Drops {
		if p.Y >= 0 && p.Y < rb.Height && p.X >= 0 && p.X < rb.Width {
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
	styles := rainColorStyles()
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

func rainColorStyles() []lipgloss.Style {
	if len(activeRainColors) == 0 {
		return []lipgloss.Style{
			lipgloss.NewStyle().Foreground(lipgloss.Color("#4488CC")),
		}
	}
	styles := make([]lipgloss.Style, len(activeRainColors))
	for i, color := range activeRainColors {
		styles[i] = lipgloss.NewStyle().Foreground(color)
	}
	return styles
}

