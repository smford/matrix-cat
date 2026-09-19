package matrix

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// DefaultSyntaxThemes lists built-in syntax palettes available for cycling.
var DefaultSyntaxThemes = []string{
	"monokai",
	"dracula",
	"nord",
	"solarized-dark",
	"github-dark",
	"fruity",
	"native",
}

// StyledRune represents a single terminal character with an optional syntax highlighting color.
type StyledRune struct {
	Rune  rune
	Color string // 24-bit Truecolor ANSI escape sequence (e.g. \x1b[38;2;...m), or empty
}

// TextBuffer manages the lines of text to be rendered, handling tab expansion,
// line wrapping, and language-aware syntax highlighting.
type TextBuffer struct {
	content       []byte
	filename      string
	rawLines      [][]StyledRune
	wrappedLines  [][]StyledRune
	lastWrapW     int
	tabWidth      int
	syntaxEnabled bool
	syntaxTheme   string
}

// NewTextBuffer creates a TextBuffer from raw content bytes with optional syntax highlighting.
func NewTextBuffer(content []byte, filename string, syntaxEnabled bool, syntaxTheme string, tabWidth int) *TextBuffer {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	if syntaxTheme == "" {
		syntaxTheme = "monokai"
	}

	tb := &TextBuffer{
		content:       content,
		filename:      filename,
		tabWidth:      tabWidth,
		syntaxEnabled: syntaxEnabled,
		syntaxTheme:   syntaxTheme,
	}
	tb.buildLines()
	return tb
}

// SetSyntax updates syntax highlighting state and theme, rebuilding line buffers.
func (tb *TextBuffer) SetSyntax(enabled bool, theme string) {
	if theme == "" {
		theme = "monokai"
	}
	tb.syntaxEnabled = enabled
	tb.syntaxTheme = theme
	tb.buildLines()
}

// buildLines parses raw content into styled rune lines based on current syntax settings.
func (tb *TextBuffer) buildLines() {
	if tb.syntaxEnabled && len(tb.content) > 0 {
		tb.rawLines = tokenizeContent(tb.filename, tb.content, tb.syntaxTheme, tb.tabWidth)
	} else {
		tb.rawLines = parsePlainLines(tb.content, tb.tabWidth)
	}
	tb.wrappedLines = nil
	tb.lastWrapW = 0
}

// tokenizeContent uses Chroma to lex and style code according to language and theme.
func tokenizeContent(filename string, content []byte, themeName string, tabWidth int) [][]StyledRune {
	var lexer chroma.Lexer
	if filename != "" && filename != "-" {
		lexer = lexers.Match(filename)
	}
	if lexer == nil {
		lexer = lexers.Analyse(string(content))
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(themeName)
	if style == nil {
		style = styles.Get("monokai")
	}
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, string(content))
	if err != nil {
		return parsePlainLines(content, tabWidth)
	}

	var rawLines [][]StyledRune
	currentLine := []StyledRune{}
	col := 0

	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := style.Get(token.Type)
		var color string
		if entry.Colour.IsSet() {
			boldPrefix := ""
			if entry.Bold == chroma.Yes {
				boldPrefix = "1;"
			}
			color = fmt.Sprintf("\x1b[%s38;2;%d;%d;%dm", boldPrefix, entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue())
		}

		runes := []rune(token.Value)
		for i := 0; i < len(runes); i++ {
			r := runes[i]
			switch r {
			case '\r':
				if i+1 < len(runes) && runes[i+1] == '\n' {
					i++
				}
				rawLines = append(rawLines, currentLine)
				currentLine = []StyledRune{}
				col = 0
			case '\n':
				rawLines = append(rawLines, currentLine)
				currentLine = []StyledRune{}
				col = 0
			case '\t':
				spaces := tabWidth - (col % tabWidth)
				for s := 0; s < spaces; s++ {
					currentLine = append(currentLine, StyledRune{Rune: ' ', Color: color})
				}
				col += spaces
			default:
				currentLine = append(currentLine, StyledRune{Rune: r, Color: color})
				col++
			}
		}
	}
	rawLines = append(rawLines, currentLine)

	if len(rawLines) > 1 && len(rawLines[len(rawLines)-1]) == 0 {
		rawLines = rawLines[:len(rawLines)-1]
	}

	return rawLines
}

// parsePlainLines converts plain text to styled runes without syntax coloring.
func parsePlainLines(content []byte, tabWidth int) [][]StyledRune {
	raw := string(content)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	splitLines := strings.Split(raw, "\n")

	if len(splitLines) > 1 && splitLines[len(splitLines)-1] == "" {
		splitLines = splitLines[:len(splitLines)-1]
	}

	lines := make([][]StyledRune, len(splitLines))
	for i, line := range splitLines {
		expanded := expandTabs(line, tabWidth)
		runes := []rune(expanded)
		styled := make([]StyledRune, len(runes))
		for j, r := range runes {
			styled[j] = StyledRune{Rune: r, Color: ""}
		}
		lines[i] = styled
	}
	return lines
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

// Wrap returns styled lines wrapped to maxWidth.
func (tb *TextBuffer) Wrap(maxWidth int) [][]StyledRune {
	if maxWidth <= 0 {
		return tb.rawLines
	}
	if tb.wrappedLines != nil && tb.lastWrapW == maxWidth {
		return tb.wrappedLines
	}

	var wrapped [][]StyledRune
	for _, line := range tb.rawLines {
		if len(line) == 0 {
			wrapped = append(wrapped, []StyledRune{})
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
