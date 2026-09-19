package matrix

import (
	"bytes"
	"context"
	"testing"
)

func TestEngineInitAndResize(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	engine := NewEngine(cfg, &buf)

	if engine.theme.Name != "green" {
		t.Errorf("expected default theme green, got %s", engine.theme.Name)
	}

	engine.resize(80, 24)
	if engine.width != 80 || engine.height != 24 {
		t.Errorf("resize mismatch: width=%d, height=%d", engine.width, engine.height)
	}
	if len(engine.columns) != 80 {
		t.Errorf("columns length mismatch: got %d, want 80", len(engine.columns))
	}

	// Shrink resize
	engine.resize(40, 10)
	if len(engine.columns) != 40 {
		t.Errorf("columns length mismatch on shrink: got %d, want 40", len(engine.columns))
	}
}

func TestEngineRender(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	engine := NewEngine(cfg, &buf)
	engine.resize(10, 5)

	// Step a few times to generate drops
	for i := 0; i < 20; i++ {
		engine.step()
	}

	engine.render()
	output := buf.String()
	if len(output) == 0 {
		t.Errorf("expected render output, got empty buffer")
	}
}

func TestEngineHandleInput(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	engine := NewEngine(cfg, &buf)

	// Test pause toggle
	if engine.paused {
		t.Errorf("initially should not be paused")
	}
	engine.handleInput(' ')
	if !engine.paused {
		t.Errorf("should be paused after pressing space")
	}
	engine.handleInput(' ')
	if engine.paused {
		t.Errorf("should be unpaused after pressing space again")
	}

	// Test theme cycling
	origTheme := engine.theme.Name
	engine.handleInput('c')
	if engine.theme.Name == origTheme {
		t.Errorf("expected theme to change after 'c'")
	}

	// Test speed and density controls
	origDensity := engine.cfg.Density
	origSpeed := engine.cfg.SpeedScale
	engine.handleInput('+')
	if engine.cfg.Density <= origDensity || engine.cfg.SpeedScale <= origSpeed {
		t.Errorf("expected density and speed to increase after '+'")
	}

	engine.handleInput('-')
	if engine.cfg.Density > origDensity || engine.cfg.SpeedScale > origSpeed {
		t.Errorf("expected density and speed to decrease after '-'")
	}

	// Test quit keys
	if !engine.handleInput('q') {
		t.Errorf("expected 'q' to return true (exit)")
	}
	if !engine.handleInput('Q') {
		t.Errorf("expected 'Q' to return true (exit)")
	}
	if !engine.handleInput(3) { // Ctrl+C
		t.Errorf("expected Ctrl+C (3) to return true (exit)")
	}
	if !engine.handleInput(27) { // ESC
		t.Errorf("expected ESC (27) to return true (exit)")
	}
}

func TestEngineTextModeRunContextCancel(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("CONTEXT CANCEL TEST")
	engine := NewEngine(cfg, &buf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := engine.Run(ctx)
	if err != nil {
		t.Errorf("expected clean exit on context cancel, got: %v", err)
	}
}

func TestEngineTextModeResize(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.FileContent = []byte("LONG LINE THAT WILL WRAP WHEN TERMINAL RESIZES TO A NARROWER WIDTH")
	engine := NewEngine(cfg, &buf)

	engine.resize(80, 24)
	if len(engine.textDrops) != 80 {
		t.Errorf("expected 80 text drops, got %d", len(engine.textDrops))
	}

	engine.resize(30, 10)
	if len(engine.textDrops) != 30 {
		t.Errorf("expected 30 text drops after resize, got %d", len(engine.textDrops))
	}
}
