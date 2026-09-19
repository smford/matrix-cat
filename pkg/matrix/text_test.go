package matrix

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewTextBuffer(t *testing.T) {
	input := "Line 1\nLine\t2\r\nLine 3\n"
	tb := NewTextBuffer([]byte(input), 4)

	if tb.RawLineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", tb.RawLineCount())
	}

	// Line 2 had a tab after "Line" (4 chars) -> next tab stop is col 4 -> 4 spaces
	line2 := string(tb.rawLines[1])
	expectedLine2 := "Line    2"
	if line2 != expectedLine2 {
		t.Errorf("tab expansion failed: got %q, want %q", line2, expectedLine2)
	}
}

func TestTextBufferWrap(t *testing.T) {
	input := "Short line\nThis is a rather long line that needs to wrap across terminal columns"
	tb := NewTextBuffer([]byte(input), 4)

	// Wrap at 20 characters
	wrapped := tb.Wrap(20)
	if len(wrapped) <= 2 {
		t.Errorf("expected more than 2 wrapped lines, got %d", len(wrapped))
	}

	for i, l := range wrapped {
		if len(l) > 20 {
			t.Errorf("line %d exceeds max width 20: len=%d (%q)", i, len(l), string(l))
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

func TestEngineTextModeRenderAndSettle(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("HELLO MATRIX\nSECOND LINE")
	cfg.ThemeName = "green"

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
