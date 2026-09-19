package matrix

import (
	"strings"
)

// TextBuffer manages the lines of text to be rendered, handling tab expansion
// and line wrapping for terminal displays.
type TextBuffer struct {
	rawLines     [][]rune
	wrappedLines [][]rune
	lastWrapW    int
	tabWidth     int
}

// NewTextBuffer creates a TextBuffer from raw content bytes.
func NewTextBuffer(content []byte, tabWidth int) *TextBuffer {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	raw := string(content)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	splitLines := strings.Split(raw, "\n")

	// Strip single trailing empty line if text ended with newline
	if len(splitLines) > 1 && splitLines[len(splitLines)-1] == "" {
		splitLines = splitLines[:len(splitLines)-1]
	}

	lines := make([][]rune, len(splitLines))
	for i, line := range splitLines {
		expanded := expandTabs(line, tabWidth)
		lines[i] = []rune(expanded)
	}

	return &TextBuffer{
		rawLines: lines,
		tabWidth: tabWidth,
	}
}

// expandTabs replaces tab characters with appropriate spaces according to tabWidth.
func expandTabs(s string, tabWidth int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var sb strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				sb.WriteByte(' ')
			}
			col += spaces
		} else {
			sb.WriteRune(r)
			col++
		}
	}
	return sb.String()
}

// Wrap returns lines wrapped to maxWidth.
func (tb *TextBuffer) Wrap(maxWidth int) [][]rune {
	if maxWidth <= 0 {
		return tb.rawLines
	}
	if tb.wrappedLines != nil && tb.lastWrapW == maxWidth {
		return tb.wrappedLines
	}

	var wrapped [][]rune
	for _, line := range tb.rawLines {
		if len(line) == 0 {
			wrapped = append(wrapped, []rune{})
			continue
		}
		if len(line) <= maxWidth {
			wrapped = append(wrapped, line)
		} else {
			rem := line
			for len(rem) > maxWidth {
				wrapped = append(wrapped, rem[:maxWidth])
				rem = rem[maxWidth:]
			}
			if len(rem) > 0 {
				wrapped = append(wrapped, rem)
			}
		}
	}

	tb.wrappedLines = wrapped
	tb.lastWrapW = maxWidth
	return wrapped
}

// TotalLines returns the number of wrapped lines for maxWidth.
func (tb *TextBuffer) TotalLines(maxWidth int) int {
	return len(tb.Wrap(maxWidth))
}

// RawLineCount returns the count of unwrapped lines.
func (tb *TextBuffer) RawLineCount() int {
	return len(tb.rawLines)
}

// TextDrop represents a falling stream in a single column designed to reveal/settle text.
type TextDrop struct {
	Y           float64 // Head position (continuous row coordinate)
	Speed       float64 // Velocity in rows per tick
	Length      int     // Trail length in rows
	Glyphs      []rune  // Matrix pool glyphs for trail
	Done        bool    // True when drop tail has passed bottom of screen
	lastHeadRow int
}

// NewTextDrop initializes a drop for text mode with staggered spawn and speed variation.
func NewTextDrop(height int, pool []rune, maxStagger int) *TextDrop {
	minLen := 8
	maxLen := 22
	if height > 10 && height < maxLen {
		maxLen = height
	}
	length := randomInt(minLen, maxLen)

	speedRoll := randomInt(1, 100)
	var speed float64
	switch {
	case speedRoll <= 30:
		speed = 0.9 + float64(randomInt(0, 50))/100.0 // Fast: 0.9 - 1.4
	case speedRoll <= 75:
		speed = 0.5 + float64(randomInt(0, 40))/100.0 // Medium: 0.5 - 0.9
	default:
		speed = 0.3 + float64(randomInt(0, 20))/100.0 // Slow: 0.3 - 0.5
	}

	if maxStagger < 0 {
		maxStagger = 25
	}
	stagger := float64(randomInt(0, maxStagger))
	initialY := -float64(length) - stagger

	glyphs := make([]rune, length)
	for i := range glyphs {
		glyphs[i] = RandomRune(pool)
	}

	return &TextDrop{
		Y:           initialY,
		Speed:       speed,
		Length:      length,
		Glyphs:      glyphs,
		lastHeadRow: int(initialY),
	}
}

// Update advances the drop's position and mutates trail characters.
func (d *TextDrop) Update(pool []rune, speedScale float64, screenHeight int) {
	if d.Done {
		return
	}
	if speedScale <= 0 {
		speedScale = 1.0
	}
	d.Y += d.Speed * speedScale
	currentHeadRow := int(d.Y)

	if currentHeadRow > d.lastHeadRow {
		steps := currentHeadRow - d.lastHeadRow
		for s := 0; s < steps; s++ {
			for i := len(d.Glyphs) - 1; i > 0; i-- {
				d.Glyphs[i] = d.Glyphs[i-1]
			}
			d.Glyphs[0] = RandomRune(pool)
		}
		d.lastHeadRow = currentHeadRow
	}

	// 5% mutation rate for trailing runes
	for i := 1; i < len(d.Glyphs); i++ {
		if randomInt(1, 100) <= 5 {
			d.Glyphs[i] = RandomRune(pool)
		}
	}

	tailRow := int(d.Y) - d.Length
	if tailRow >= screenHeight {
		d.Done = true
	}
}
