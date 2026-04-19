package ui

import (
	"math/rand"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

type snowTree struct {
	x          int
	h          int
	frost      int
	birthFrame int
}

type snowSmoke struct {
	x, y int
	age  int
}

const (
	snowmanPhaseNone = iota
	snowmanPhaseBaseDot
	snowmanPhaseBaseSmall
	snowmanPhaseBaseMed
	snowmanPhaseBaseLarge
	snowmanPhaseHeadDot
	snowmanPhaseHeadRound
	snowmanPhaseFace
	snowmanPhasePipe
	snowmanPhaseHat
)

const snowCabinW = 7

var snowflakeChars = [...]string{"·", "˙", "⁚", "*", "`", ","}

func (rb *RainBackground) initSnowScene() {
	if rb.Width <= 0 {
		return
	}
	rb.SnowGround = make([]int, rb.Width)
	rb.SnowTrees = rb.SnowTrees[:0]
	rb.SnowSmoke = rb.SnowSmoke[:0]
	rb.SnowmanPhase = snowmanPhaseNone
	rb.SnowmanX = 0
	rb.SnowmanBuild = 0
	rb.SnowmanAux = 0
	rb.SnowCabinFrost = 0
	rb.SnowCabinLeft = rb.Width/2 - snowCabinW/2
	if rb.SnowCabinLeft < 0 {
		rb.SnowCabinLeft = 0
	}
	if rb.SnowCabinLeft+snowCabinW > rb.Width {
		rb.SnowCabinLeft = rb.Width - snowCabinW
		if rb.SnowCabinLeft < 0 {
			rb.SnowCabinLeft = 0
		}
	}
	if rb.Width > 16 {
		rb.SnowmanX = rb.Width - 5
	} else {
		rb.SnowmanX = rb.SnowCabinLeft - 4
		if rb.SnowmanX < 2 {
			rb.SnowmanX = min(rb.Width-3, rb.SnowCabinLeft+snowCabinW+3)
		}
	}
	rb.snowSpawnInitialTrees()
}

// snowTreeSiteFree reports whether a trunk at column x can sit without clipping
// the 3-cell canopy (/\, /█\, or trunk) and without overlapping the cabin or
// snowman reservation.
func (rb *RainBackground) snowTreeSiteFree(x int) bool {
	if x < 1 || x >= rb.Width-1 {
		return false
	}
	return rb.snowFootprintFree(x-1, 3)
}

// snowSpawnInitialTrees places scattered evergreens with random trunk columns.
// Target count and minimum spacing scale with width and height; each new
// RainBackground (e.g. resize) picks a fresh layout.
func (rb *RainBackground) snowSpawnInitialTrees() {
	if rb.Width < 10 || rb.Height < 5 {
		return
	}
	want := rb.Width/9 + rb.Height/3
	if want < 2 {
		want = 2
	}
	if want > 14 {
		want = 14
	}
	minGap := rb.Width / 12
	if minGap < 4 {
		minGap = 4
	}
	if minGap > 10 {
		minGap = 10
	}

	const maxAttempts = 500
	trees := make([]snowTree, 0, want)

	for attempt := 0; attempt < maxAttempts && len(trees) < want; attempt++ {
		x := 1 + rand.Intn(rb.Width-2)
		if !rb.snowTreeSiteFree(x) {
			continue
		}
		tooClose := false
		for _, tr := range trees {
			if absInt(tr.x-x) < minGap {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		h := 2
		if rb.Height >= 9 && rand.Intn(100) < 55 {
			h = 3
		}
		frost := rand.Intn(3)
		birth := rand.Intn(67)
		trees = append(trees, snowTree{x: x, h: h, frost: frost, birthFrame: birth})
	}

	if len(trees) == 0 {
		for x := 1; x < rb.Width-1; x++ {
			if rb.snowTreeSiteFree(x) {
				h := 2
				if rb.Height >= 9 {
					h = 3
				}
				trees = append(trees, snowTree{x: x, h: h, frost: 0, birthFrame: 0})
				break
			}
		}
	}

	rb.SnowTrees = append(rb.SnowTrees[:0], trees...)
}

func (rb *RainBackground) snowChimneyTop() (int, int) {
	roofY := rb.Height - 4
	if roofY < 1 {
		roofY = 1
	}
	cx := rb.SnowCabinLeft + 5
	if cx >= rb.Width {
		cx = rb.Width - 1
	}
	return cx, roofY - 1
}

func (rb *RainBackground) snowNoteFlakeLand(x int) {
	if rb.Mode != config.UIRainAnimationSnow || rb.SnowGround == nil {
		return
	}
	if rb.SnowmanPhase != snowmanPhaseNone && rb.SnowmanPhase < snowmanPhaseFace {
		d := absInt(x - rb.SnowmanX)
		if d <= 2 {
			rb.SnowmanBuild++
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (rb *RainBackground) snowTotalGround() int {
	if rb.SnowGround == nil {
		return 0
	}
	t := 0
	for _, v := range rb.SnowGround {
		t += v
	}
	return t
}

func (rb *RainBackground) snowAdvanceScene() {
	if rb.Width <= 0 || rb.Height <= 0 {
		return
	}

	if rb.Frame%40 == 0 && rb.SnowCabinFrost < 3 {
		rb.SnowCabinFrost++
	}
	for i := range rb.SnowTrees {
		if rb.SnowTrees[i].frost >= 3 {
			continue
		}
		if (rb.Frame+rb.SnowTrees[i].birthFrame)%55 == 0 {
			rb.SnowTrees[i].frost++
		}
	}

	if rb.SnowmanPhase == snowmanPhaseNone && rb.Width >= 18 {
		if rb.snowTotalGround() >= rb.Width*4 {
			rb.SnowmanPhase = snowmanPhaseBaseDot
			rb.SnowmanBuild = 0
			rb.SnowmanAux = 0
		}
	}

	switch rb.SnowmanPhase {
	case snowmanPhaseBaseDot:
		if rb.SnowmanBuild >= 4 {
			rb.SnowmanPhase = snowmanPhaseBaseSmall
			rb.SnowmanBuild = 0
		}
	case snowmanPhaseBaseSmall:
		if rb.SnowmanBuild >= 6 {
			rb.SnowmanPhase = snowmanPhaseBaseMed
			rb.SnowmanBuild = 0
		}
	case snowmanPhaseBaseMed:
		if rb.SnowmanBuild >= 8 {
			rb.SnowmanPhase = snowmanPhaseBaseLarge
			rb.SnowmanBuild = 0
		}
	case snowmanPhaseBaseLarge:
		if rb.SnowmanBuild >= 6 {
			rb.SnowmanPhase = snowmanPhaseHeadDot
			rb.SnowmanBuild = 0
		}
	case snowmanPhaseHeadDot:
		if rb.SnowmanBuild >= 5 {
			rb.SnowmanPhase = snowmanPhaseHeadRound
			rb.SnowmanBuild = 0
		}
	case snowmanPhaseHeadRound:
		if rb.SnowmanBuild >= 8 {
			rb.SnowmanPhase = snowmanPhaseFace
			rb.SnowmanBuild = 0
			rb.SnowmanAux = 0
		}
	case snowmanPhaseFace:
		rb.SnowmanAux++
		if rb.SnowmanAux >= 48 {
			rb.SnowmanPhase = snowmanPhasePipe
			rb.SnowmanAux = 0
		}
	case snowmanPhasePipe:
		rb.SnowmanAux++
		if rb.SnowmanAux >= 32 {
			rb.SnowmanPhase = snowmanPhaseHat
			rb.SnowmanAux = 0
		}
	case snowmanPhaseHat:
		// terminal
	}

	if rb.Frame%8 == 0 {
		cx, cy := rb.snowChimneyTop()
		if cy >= 0 && cx >= 0 && cx < rb.Width {
			rb.SnowSmoke = append(rb.SnowSmoke, snowSmoke{x: cx, y: cy, age: 0})
		}
	}
	aliveSmoke := rb.SnowSmoke[:0]
	for _, s := range rb.SnowSmoke {
		ns := s
		ns.age++
		if ns.age%2 == 0 {
			ns.y--
		}
		if rand.Float64() < 0.35 {
			ns.x += rand.Intn(3) - 1
		}
		if ns.x < 0 {
			ns.x = 0
		}
		if ns.x >= rb.Width {
			ns.x = rb.Width - 1
		}
		if ns.y >= 0 && ns.age < 28 {
			aliveSmoke = append(aliveSmoke, ns)
		}
	}
	rb.SnowSmoke = aliveSmoke
}

func (rb *RainBackground) snowFootprintFree(x, w int) bool {
	left := x
	right := x + w
	cL, cR := rb.SnowCabinLeft-1, rb.SnowCabinLeft+snowCabinW+2
	if right > cL && left < cR {
		return false
	}
	sL, sR := rb.SnowmanX-3, rb.SnowmanX+4
	if rb.SnowmanPhase >= snowmanPhaseBaseLarge {
		sL, sR = rb.SnowmanX-5, rb.SnowmanX+5
	}
	if rb.SnowmanPhase != snowmanPhaseNone && right > sL && left < sR {
		return false
	}
	return true
}

func snowGroundGlyph(depth int) string {
	switch {
	case depth < 1:
		return " "
	case depth < 6:
		return "·"
	case depth < 18:
		return "░"
	case depth < 40:
		return "▒"
	default:
		return "▓"
	}
}

func (rb *RainBackground) snowPaintCell(cells []string, x, y int, ch string, st lipgloss.Style) {
	if x < 0 || x >= rb.Width || y < 0 || y >= rb.Height {
		return
	}
	idx := y*rb.Width + x
	if idx >= 0 && idx < len(cells) {
		cells[idx] = st.Render(ch)
	}
}

func (rb *RainBackground) snowPaintLine(cells []string, left, y int, line string, st lipgloss.Style) {
	col := 0
	for _, r := range line {
		x := left + col
		if x >= 0 && x < rb.Width {
			rb.snowPaintCell(cells, x, y, string(r), st)
		}
		col++
	}
}

func (rb *RainBackground) paintSnowScene(cells []string) {
	if rb.Width <= 0 || rb.Height <= 0 || rb.SnowGround == nil {
		return
	}
	night := lipgloss.NewStyle().Foreground(lipgloss.Color("#1A237E")).Faint(true)
	star := lipgloss.NewStyle().Foreground(lipgloss.Color("#B0BEC5"))
	wood := lipgloss.NewStyle().Foreground(lipgloss.Color("#5D4037"))
	woodHi := lipgloss.NewStyle().Foreground(lipgloss.Color("#6D4C41"))
	win := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC80")).Bold(true)
	ever := lipgloss.NewStyle().Foreground(lipgloss.Color("#1B5E20"))
	everFrost := lipgloss.NewStyle().Foreground(lipgloss.Color("#C8E6C9")).Faint(true)
	// Snowman: high-contrast whites + bold so the figure reads above the ground bank.
	snowBall := lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Bold(true)
	snowHi := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	snowParen := lipgloss.NewStyle().Foreground(lipgloss.Color("#90A4AE")).Bold(true)
	coal := lipgloss.NewStyle().Foreground(lipgloss.Color("#37474F")).Bold(true)
	nose := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9100")).Bold(true)
	smokeSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#90A4AE")).Faint(true)
	hatSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#212121")).Bold(true)
	frost := lipgloss.NewStyle().Foreground(lipgloss.Color("#ECEFF1")).Faint(true)

	// Night sky (row 0)
	for x := 0; x < rb.Width; x++ {
		var ch string
		if (x+rb.Frame)%6 == 0 && (x*5+rb.Frame)%13 < 3 {
			ch = star.Render("·")
		} else if (x+rb.Frame*2)%9 == 0 {
			ch = night.Render("░")
		} else {
			ch = night.Render(" ")
		}
		cells[x] = ch
	}

	// Trees (evergreens: full silhouette from frame one)
	for _, tr := range rb.SnowTrees {
		baseY := rb.Height - 2
		st := ever
		if tr.frost >= 2 {
			st = everFrost
		}
		for k := 0; k < tr.h; k++ {
			y := baseY - k
			if y < 1 || y >= rb.Height {
				continue
			}
			if k == tr.h-1 {
				rb.snowPaintLine(cells, tr.x-1, y, "/\\", st)
				continue
			}
			if k == tr.h-2 && tr.h >= 3 {
				rb.snowPaintLine(cells, tr.x-1, y, "/█\\", st)
				continue
			}
			rb.snowPaintCell(cells, tr.x, y, "┃", st)
		}
	}

	// Cabin (3 rows) when there is vertical room
	roofY := rb.Height - 4
	midY := rb.Height - 3
	botY := rb.Height - 2
	if roofY >= 1 && midY >= 2 && botY >= 3 {
		L := rb.SnowCabinLeft
		roof := " /---^ "
		if rb.SnowCabinFrost >= 2 {
			roof = " /~+~^ "
		}
		rb.snowPaintLine(cells, L, roofY, roof, woodHi)
		mid := "| * * |"
		midRunes := []rune(mid)
		for xi := 0; xi < len(midRunes) && xi < snowCabinW; xi++ {
			c := string(midRunes[xi])
			st := wood
			if c == "*" {
				st = win
			}
			rb.snowPaintCell(cells, L+xi, midY, c, st)
		}
		bot := "|_____|"
		rb.snowPaintLine(cells, L, botY, bot, woodHi)
		if rb.SnowCabinFrost >= 1 {
			roofRunes := []rune(roof)
			for xi := 0; xi < len(roofRunes) && xi < snowCabinW; xi++ {
				xp := L + xi
				if xp == L || xp == L+snowCabinW-1 {
					rb.snowPaintCell(cells, xp, roofY, string(roofRunes[xi]), frost)
				}
			}
		}
		chx := rb.SnowCabinLeft + 5
		if chx < rb.Width && roofY-1 >= 0 {
			rb.snowPaintCell(cells, chx, roofY-1, "█", woodHi)
		}
	}

	// Ground snow
	gy := rb.Height - 1
	for x := 0; x < rb.Width && x < len(rb.SnowGround); x++ {
		d := rb.SnowGround[x]
		g := snowGroundGlyph(d)
		st := lipgloss.NewStyle().Foreground(lipgloss.Color("#E3F2FD"))
		if d >= 18 {
			st = lipgloss.NewStyle().Foreground(lipgloss.Color("#BBDEFB"))
		}
		if d >= 40 {
			st = lipgloss.NewStyle().Foreground(lipgloss.Color("#90CAF9"))
		}
		rb.snowPaintCell(cells, x, gy, g, st)
		if d >= 55 && gy-1 >= 1 {
			rb.snowPaintCell(cells, x, gy-1, "░", st.Faint(true))
		}
	}

	// Snowman — feet sit one row above the ground snow bank (row h-2 vs pile h-1)
	// so the figure reads clearly and is not merged into the depth glyphs.
	feetY := rb.Height - 2
	bellyY := feetY - 1
	faceY := feetY - 2
	scarfY := feetY - 3
	hatBrimY := feetY - 4
	hatTopY := feetY - 5
	scarfSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#E53935")).Bold(true)

	if rb.SnowmanPhase >= snowmanPhaseBaseDot {
		switch {
		case rb.SnowmanPhase >= snowmanPhaseBaseLarge:
			rb.snowPaintCell(cells, rb.SnowmanX-1, feetY, "(", snowParen)
			rb.snowPaintCell(cells, rb.SnowmanX, feetY, "●", snowBall)
			rb.snowPaintCell(cells, rb.SnowmanX+1, feetY, ")", snowParen)
		case rb.SnowmanPhase >= snowmanPhaseBaseMed:
			rb.snowPaintCell(cells, rb.SnowmanX-1, feetY, "(", snowParen)
			rb.snowPaintCell(cells, rb.SnowmanX, feetY, "○", snowBall)
			rb.snowPaintCell(cells, rb.SnowmanX+1, feetY, ")", snowParen)
		case rb.SnowmanPhase >= snowmanPhaseBaseSmall:
			rb.snowPaintCell(cells, rb.SnowmanX, feetY, "○", snowBall)
		default:
			rb.snowPaintCell(cells, rb.SnowmanX, feetY, "·", snowHi)
		}
	}
	if rb.SnowmanPhase >= snowmanPhaseHeadDot && bellyY >= 1 {
		if rb.SnowmanPhase >= snowmanPhaseHeadRound {
			rb.snowPaintCell(cells, rb.SnowmanX, bellyY, "●", snowHi)
		} else {
			rb.snowPaintCell(cells, rb.SnowmanX, bellyY, "•", snowBall)
		}
	}
	if rb.SnowmanPhase >= snowmanPhaseFace && faceY >= 1 {
		rb.snowPaintCell(cells, rb.SnowmanX-1, faceY, "●", coal)
		rb.snowPaintCell(cells, rb.SnowmanX+1, faceY, "●", coal)
		rb.snowPaintCell(cells, rb.SnowmanX, faceY, "▲", nose)
	}
	if rb.SnowmanPhase >= snowmanPhasePipe && faceY >= 1 {
		rb.snowPaintCell(cells, rb.SnowmanX-2, faceY, "╴", woodHi)
		rb.snowPaintCell(cells, rb.SnowmanX+2, faceY, "╶", woodHi)
	}
	if rb.SnowmanPhase >= snowmanPhasePipe && bellyY >= 1 {
		rb.snowPaintCell(cells, rb.SnowmanX-2, bellyY, "╱", snowBall)
		rb.snowPaintCell(cells, rb.SnowmanX+2, bellyY, "╲", snowBall)
	}
	if rb.SnowmanPhase >= snowmanPhaseHat && scarfY >= 1 {
		rb.snowPaintLine(cells, rb.SnowmanX-1, scarfY, "≋≋≋", scarfSt)
	}
	if rb.SnowmanPhase >= snowmanPhaseHat && hatBrimY >= 1 {
		rb.snowPaintLine(cells, rb.SnowmanX-1, hatBrimY, "───", hatSt)
	}
	if rb.SnowmanPhase >= snowmanPhaseHat && hatTopY >= 1 {
		rb.snowPaintCell(cells, rb.SnowmanX, hatTopY, "█", hatSt)
		if hatTopY-1 >= 1 {
			rb.snowPaintCell(cells, rb.SnowmanX, hatTopY-1, "●", hatSt.Foreground(lipgloss.Color("#B71C1C")))
		}
	}

	for _, sm := range rb.SnowSmoke {
		if sm.y >= 0 && sm.y < rb.Height {
			rb.snowPaintCell(cells, sm.x, sm.y, "░", smokeSt)
		}
	}
}
