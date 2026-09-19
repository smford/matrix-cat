package matrix

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewTextBuffer(t *testing.T) {
	input := "Line 1\nLine\t2\r\nLine 3\n"
	tb := NewTextBuffer([]byte(input), "test.txt", false, "monokai", 4)

	if tb.RawLineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", tb.RawLineCount())
	}

	// Line 2 had a tab after "Line" (4 chars) -> next tab stop is col 4 -> 4 spaces
	var runes []rune
	for _, sr := range tb.rawLines[1] {
		runes = append(runes, sr.Rune)
	}
	line2 := string(runes)
	expectedLine2 := "Line    2"
	if line2 != expectedLine2 {
		t.Errorf("tab expansion failed: got %q, want %q", line2, expectedLine2)
	}
}

func TestTextBufferWrap(t *testing.T) {
	input := "Short line\nThis is a rather long line that needs to wrap across terminal columns"
	tb := NewTextBuffer([]byte(input), "test.txt", false, "monokai", 4)

	// Wrap at 20 characters
	wrapped := tb.Wrap(20)
	if len(wrapped) <= 2 {
		t.Errorf("expected more than 2 wrapped lines, got %d", len(wrapped))
	}

	for i, l := range wrapped {
		if len(l) > 20 {
			t.Errorf("line %d exceeds max width 20: len=%d", i, len(l))
		}
	}

	// Wrap with maxWidth <= 0 returns rawLines
	raw := tb.Wrap(0)
	if len(raw) != 2 {
		t.Errorf("expected 2 raw lines when maxWidth=0, got %d", len(raw))
	}
}

func TestNewTextDrop(t *testing.T) {
	pool := []rune{'a', 'b', 'c', 'd'}
	height := 30
	drop := NewTextDrop(height, pool, 10)

	if drop.Length < 8 || drop.Length > 22 {
		t.Errorf("expected drop length in [8, 22], got %d", drop.Length)
	}
	if drop.Speed <= 0 {
		t.Errorf("expected positive speed, got %f", drop.Speed)
	}
	if drop.Y >= 0 {
		t.Errorf("drop should spawn above screen (Y < 0), got %f", drop.Y)
	}
	if drop.Done {
		t.Errorf("new drop should not be marked Done")
	}

	// Test Update advancing Y
	prevY := drop.Y
	drop.Update(pool, 1.0, height)
	if drop.Y <= prevY {
		t.Errorf("drop Y should advance after Update, got %f <= %f", drop.Y, prevY)
	}

	// Test marking Done when tail passes height
	drop.Y = float64(height + drop.Length + 5)
	drop.Update(pool, 1.0, height)
	if !drop.Done {
		t.Errorf("drop should be marked Done when tail passes height")
	}
}

func TestSyntaxHighlighting(t *testing.T) {
	goCode := `package main

import "fmt"

// main entry point
func main() {
	x := 42
	fmt.Println("Matrix", x)
}
`
	tb := NewTextBuffer([]byte(goCode), "main.go", true, "monokai", 4)
	if tb.RawLineCount() == 0 {
		t.Fatalf("expected parsed lines, got 0")
	}

	// Verify keywords and literals have ANSI color codes attached
	hasColoredRunes := false
	for _, line := range tb.rawLines {
		for _, sr := range line {
			if sr.Color != "" && strings.Contains(sr.Color, "\x1b[") {
				hasColoredRunes = true
				break
			}
		}
		if hasColoredRunes {
			break
		}
	}

	if !hasColoredRunes {
		t.Errorf("expected syntax highlighted runes to have ANSI colors, found none")
	}

	// Test SetSyntax toggle
	tb.SetSyntax(false, "monokai")
	allBlankColors := true
	for _, line := range tb.rawLines {
		for _, sr := range line {
			if sr.Color != "" {
				allBlankColors = false
				break
			}
		}
	}
	if !allBlankColors {
		t.Errorf("expected all colors to be empty when syntax highlighting is disabled")
	}
}

func TestEngineTextModeRenderAndSettle(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("HELLO MATRIX\nSECOND LINE")
	cfg.ThemeName = "green"
	cfg.SyntaxHighlight = false

	engine := NewEngine(cfg, &buf)
	if !engine.isTextMode() {
		t.Fatalf("expected engine to be in text mode")
	}

	engine.resize(40, 10)

	if len(engine.textDrops) != 40 {
		t.Fatalf("expected 40 text drops for width 40, got %d", len(engine.textDrops))
	}

	// Fast forward to settle all characters
	engine.fastForwardTextRain()
	if !engine.allTextDropsDone() {
		t.Errorf("expected all text drops to be marked done after fast-forward")
	}

	engine.render()
	output := buf.String()
	cleanOutput := stripANSI(output)
	if !strings.Contains(cleanOutput, "HELLO MATRIX") {
		t.Errorf("settled render output should contain 'HELLO MATRIX', got:\n%s", cleanOutput)
	}
	if !strings.Contains(cleanOutput, "SECOND LINE") {
		t.Errorf("settled render output should contain 'SECOND LINE', got:\n%s", cleanOutput)
	}
}

func TestEngineSyntaxToggleAndThemeCycle(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FilePath = "main.go"
	cfg.FileContent = []byte("package main\nfunc test() {}")
	cfg.SyntaxHighlight = true
	cfg.SyntaxTheme = "monokai"

	engine := NewEngine(cfg, &buf)
	engine.resize(40, 10)
	engine.fastForwardTextRain()

	if !engine.syntaxHighlight {
		t.Errorf("expected syntaxHighlight to be initially true")
	}

	// Toggle off with 's'
	engine.handleInput('s')
	if engine.syntaxHighlight {
		t.Errorf("expected syntaxHighlight to be false after 's'")
	}

	// Toggle on with 's'
	engine.handleInput('s')
	if !engine.syntaxHighlight {
		t.Errorf("expected syntaxHighlight to be true after second 's'")
	}

	// Cycle theme with 't'
	initialTheme := engine.cfg.SyntaxTheme
	engine.handleInput('t')
	if engine.cfg.SyntaxTheme == initialTheme {
		t.Errorf("expected syntax theme to cycle after 't'")
	}
}

func TestEngineTextModeCentering(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("CENTERED")
	cfg.Center = true

	engine := NewEngine(cfg, &buf)
	engine.resize(30, 10)

	engine.fastForwardTextRain()
	engine.render()
	output := buf.String()

	if !strings.Contains(output, "CENTERED") {
		t.Errorf("output should contain 'CENTERED'")
	}
}

func TestEngineTextModeScrolling(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	// 10 lines of text
	content := "Line 0\nLine 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9"
	cfg.FileContent = []byte(content)

	engine := NewEngine(cfg, &buf)
	// Terminal height 4
	engine.resize(20, 4)

	maxScroll := engine.maxScrollOffset()
	if maxScroll != 6 {
		t.Errorf("expected maxScrollOffset=6 (10-4), got %d", maxScroll)
	}

	// Initially at top (offset 0)
	if engine.scrollOffset != 0 {
		t.Errorf("initial scrollOffset should be 0, got %d", engine.scrollOffset)
	}

	// Scroll down 2
	engine.handleInput('j')
	engine.handleInput('j')
	if engine.scrollOffset != 2 {
		t.Errorf("expected scrollOffset=2 after 2x 'j', got %d", engine.scrollOffset)
	}

	// Scroll up 1
	engine.handleInput('k')
	if engine.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after 'k', got %d", engine.scrollOffset)
	}

	// Scroll to end
	engine.handleInput('G')
	if engine.scrollOffset != maxScroll {
		t.Errorf("expected scrollOffset=%d after 'G', got %d", maxScroll, engine.scrollOffset)
	}

	// Scroll to top
	engine.handleInput('g')
	if engine.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after 'g', got %d", engine.scrollOffset)
	}
}

func TestEngineTextModeFastForwardKey(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("HELLO")
	engine := NewEngine(cfg, &buf)
	engine.resize(20, 10)

	// Drops are initially not done
	if engine.allTextDropsDone() {
		t.Errorf("drops should not be done initially")
	}

	// Press Enter ('\n')
	engine.handleInput('\n')

	if !engine.allTextDropsDone() {
		t.Errorf("Enter key should fast-forward and mark all text drops done")
	}
}

func TestThemeSettledColor(t *testing.T) {
	themes := []string{"green", "cyan", "amber", "red", "white", "rainbow"}
	for _, name := range themes {
		th := GetThemeByName(name)
		col := th.SettledColor(0)
		if len(col) == 0 || !strings.Contains(col, "\x1b[") {
			t.Errorf("theme %s SettledColor() returned invalid ANSI: %q", name, col)
		}
	}
}

func TestEngineSearch(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	content := "first target match\nsecond line\nthird TARGET match\nfourth line\nfifth target match"
	cfg.FileContent = []byte(content)

	engine := NewEngine(cfg, &buf)
	engine.resize(40, 10)

	// Execute search for "target" (case-insensitive)
	engine.searchQuery = "target"
	engine.executeSearch()

	if len(engine.searchMatches) != 3 {
		t.Fatalf("expected 3 matches for 'target', got %d", len(engine.searchMatches))
	}
	if engine.currentMatch != 0 {
		t.Errorf("expected currentMatch to start at 0, got %d", engine.currentMatch)
	}

	// Active match check
	firstMatch := engine.searchMatches[0]
	matched, active := engine.isMatch(firstMatch.Line, firstMatch.Col)
	if !matched || !active {
		t.Errorf("expected first match to be matched and active, got matched=%v, active=%v", matched, active)
	}

	// Next match with 'n'
	engine.handleInput('n')
	if engine.currentMatch != 1 {
		t.Errorf("expected currentMatch=1 after 'n', got %d", engine.currentMatch)
	}

	// Next match with 'n' again
	engine.handleInput('n')
	if engine.currentMatch != 2 {
		t.Errorf("expected currentMatch=2 after second 'n', got %d", engine.currentMatch)
	}

	// Next match with 'n' wraps around to 0
	engine.handleInput('n')
	if engine.currentMatch != 0 {
		t.Errorf("expected currentMatch=0 after wrap-around 'n', got %d", engine.currentMatch)
	}

	// Previous match with 'p' wraps around to 2
	engine.handleInput('p')
	if engine.currentMatch != 2 {
		t.Errorf("expected currentMatch=2 after 'p' wrap-around, got %d", engine.currentMatch)
	}

	// Previous match with 'p' goes to 1
	engine.handleInput('p')
	if engine.currentMatch != 1 {
		t.Errorf("expected currentMatch=1 after 'p', got %d", engine.currentMatch)
	}
}

func TestEngineSearchInteractiveInput(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	content := "hello world\nmatrix digital rain\nsearch test"
	cfg.FileContent = []byte(content)

	engine := NewEngine(cfg, &buf)
	engine.resize(40, 10)

	// Type '/' to start search
	engine.handleInput('/')
	if !engine.searching {
		t.Errorf("expected engine.searching to be true after '/'")
	}

	// Type "rain" with a typo: 'r', 'a', 'i', 'x', Backspace, 'n'
	engine.handleInput('r')
	engine.handleInput('a')
	engine.handleInput('i')
	engine.handleInput('x')
	if engine.searchQuery != "raix" {
		t.Errorf("expected searchQuery 'raix', got %q", engine.searchQuery)
	}

	// Backspace
	engine.handleInput(127)
	if engine.searchQuery != "rai" {
		t.Errorf("expected searchQuery 'rai' after backspace, got %q", engine.searchQuery)
	}

	engine.handleInput('n')
	if engine.searchQuery != "rain" {
		t.Errorf("expected searchQuery 'rain', got %q", engine.searchQuery)
	}

	// Press Enter to confirm search
	engine.handleInput('\r')
	if engine.searching {
		t.Errorf("expected engine.searching to be false after Enter")
	}
	if engine.activeQuery != "rain" {
		t.Errorf("expected activeQuery 'rain', got %q", engine.activeQuery)
	}
	if len(engine.searchMatches) != 1 {
		t.Fatalf("expected 1 match for 'rain', got %d", len(engine.searchMatches))
	}

	// Press Esc to clear search
	engine.handleInput(27)
	if engine.activeQuery != "" || len(engine.searchMatches) != 0 {
		t.Errorf("expected Esc to clear active search query and matches")
	}
}

func TestEngineSearchPatternNotFound(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("short content")

	engine := NewEngine(cfg, &buf)
	engine.resize(40, 10)

	engine.searchQuery = "nonexistent_pattern"
	engine.executeSearch()

	if len(engine.searchMatches) != 0 {
		t.Errorf("expected 0 matches for nonexistent pattern, got %d", len(engine.searchMatches))
	}
	if !strings.Contains(engine.searchStatus, "Pattern not found") {
		t.Errorf("expected 'Pattern not found' in searchStatus, got %q", engine.searchStatus)
	}
}

func TestEngineSearchRenderHighlight(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("func main() {\n\treturn 42\n}")
	cfg.SyntaxHighlight = false

	engine := NewEngine(cfg, &buf)
	engine.resize(30, 8)

	engine.searchQuery = "main"
	engine.executeSearch()

	engine.render()
	output := buf.String()

	// Output should contain active search highlight color 255;235;59
	if !strings.Contains(output, "255;235;59") {
		t.Errorf("expected active search highlight ANSI sequence (255;235;59) in render output")
	}

	// Output should contain search status bar
	if !strings.Contains(output, "[1/1] /main") {
		t.Errorf("expected search status bar '[1/1] /main' in render output")
	}
}

func stripANSI(s string) string {
	var sb strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || s[i] == '~' {
				inEsc = false
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}
