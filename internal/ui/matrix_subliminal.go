package ui

import "strings"

// matrixSubliminalPhrases are short ASCII marquees (one terminal cell per rune)
// woven into matrix mode only — anti-hero energy, confidence, and dumb jokes.
var matrixSubliminalPhrases = []string{
	"THEY CHOSE THE WRONG DAY",
	"NOT THE HERO TYPE TODAY",
	"STAND IN THE WAY ANYWAY",
	"WRONG PERSON WRONG PLAN",
	"TRY ME",
	"WATCH THIS",
	"STILL STANDING",
	"ENOUGH",
	"RISE ANYWAY",
	"NEVER AGAIN",
	"ON PURPOSE",
	"NO APOLOGY NEEDED",
	"NOT FORGOTTEN",
	"I REMEMBER",
	"JUSTICE IS LOUD",
	"BE YOUR OWN BACKUP PLAN",
	"FINE I WILL DO IT",
	"GIT PUSH FORCE OF WILL",
	"MERGE CONFLICT IN MY SOUL",
	"IT COMPILED IN SPIRIT",
	"COMPILER HATES YOU TOO",
	"HYDRATE OR DIEDRATE",
	"BE KIND THEN TAKE NAMES",
	"PIZZA IS A LIFETIME COMMIT",
	"NOT TODAY SATAN",
	"CONFIDENCE CLIPPED AT ZERO",
	"RUN IT TWICE",
	"NO REGRETS ONLY REBASES",
}

// matrixSubliminalStream is all phrases joined for slow single-column crawls.
var matrixSubliminalStream = buildMatrixSubliminalStream()

func buildMatrixSubliminalStream() string {
	var b strings.Builder
	for _, p := range matrixSubliminalPhrases {
		if p == "" {
			continue
		}
		b.WriteString(p)
		b.WriteByte(' ')
	}
	return b.String()
}

// matrixVerticalSubliminalChar shows one letter at a time from the stream
// (matrix wave: occasional faint glyph in a fixed column).
func matrixVerticalSubliminalChar(frame int) (ch string, ok bool) {
	s := matrixSubliminalStream
	if s == "" {
		return "", false
	}
	const hold = 20
	t := frame / hold
	runes := []rune(s)
	if len(runes) == 0 {
		return "", false
	}
	for k := 0; k < len(runes); k++ {
		r := runes[(t+k)%len(runes)]
		if r != ' ' {
			return string(r), true
		}
	}
	return "", false
}

// matrixMarqueeChar returns a single visible character for column x when the
// scrolling phrase covers that cell; otherwise ok is false.
func matrixMarqueeChar(x, frame, width int) (ch string, ok bool) {
	if width < 1 || len(matrixSubliminalPhrases) == 0 {
		return "", false
	}
	const gap = 55
	phrase := matrixSubliminalPhrases[frame/matrixMarqueePhraseHoldFrames%len(matrixSubliminalPhrases)]
	if len(phrase) == 0 {
		return "", false
	}
	cycle := len(phrase) + width + gap
	t := frame % cycle
	startCol := width - t
	rel := x - startCol
	if rel < 0 || rel >= len(phrase) {
		return "", false
	}
	c := phrase[rel]
	if c == ' ' {
		return "", false
	}
	return string(c), true
}

// matrixMarqueePhraseHoldFrames is how many frames each phrase stays before the
// scroll cycle advances to the next line in the list.
const matrixMarqueePhraseHoldFrames = 420

// matrixSubliminalBackgroundRow picks at most one row in the rain field for a
// fainter second marquee (different phase so it rarely lines up with the wave).
func matrixSubliminalBackgroundRow(height int) int {
	if height < 3 {
		return -1
	}
	return height / 2
}

func matrixMarqueeCharBackground(x, frame, width, height int) (ch string, ok bool) {
	row := matrixSubliminalBackgroundRow(height)
	if row < 0 {
		return "", false
	}
	// Offset phase so the background line is not synced with the wave strip.
	phase := frame + width/2 + row*17
	return matrixMarqueeChar(x, phase, width)
}

// matrixWaveMaybeSubliminal replaces a wave cell with a faint marquee letter
// when the scroll window covers that column (low frequency via phase primes).
func matrixWaveMaybeSubliminal(x, frame, width int) (ch string, ok bool) {
	if (x+frame*3)%11 != 0 && (x+frame)%13 != 0 {
		return "", false
	}
	return matrixMarqueeChar(x, frame+width+29, width)
}
