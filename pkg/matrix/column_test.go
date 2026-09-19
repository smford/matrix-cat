package matrix

import (
	"testing"
)

func TestNewDrop(t *testing.T) {
	pool := []rune{'a', 'b', 'c'}
	height := 40
	drop := NewDrop(height, pool)

	if drop.Length < 6 || drop.Length > 28 {
		t.Errorf("drop length %d out of expected range [6, 28]", drop.Length)
	}
	if len(drop.Glyphs) != drop.Length {
		t.Errorf("glyphs count %d does not match length %d", len(drop.Glyphs), drop.Length)
	}
	if drop.Speed <= 0 {
		t.Errorf("drop speed must be positive, got %f", drop.Speed)
	}
	if drop.Y >= 0 {
		t.Errorf("drop should initialize above screen, got Y=%f", drop.Y)
	}
}

func TestDropUpdateAndOffScreen(t *testing.T) {
	pool := []rune{'a', 'b', 'c'}
	drop := NewDrop(20, pool)
	initialY := drop.Y

	drop.Update(pool, 1.0)
	if drop.Y <= initialY {
		t.Errorf("drop Y should advance after Update, got %f <= %f", drop.Y, initialY)
	}

	// Move drop well past bottom of screen
	drop.Y = 100
	if !drop.IsOffScreen(50) {
		t.Errorf("drop should be marked off-screen when head is at 100 and height is 50")
	}
}

func TestColumnLifecycle(t *testing.T) {
	pool := []rune{'a', 'b', 'c'}
	col := NewColumn()

	if len(col.Drops) != 0 {
		t.Fatalf("expected empty drops in initial column")
	}

	// Force spawn with 100% density
	for i := 0; i < 50; i++ {
		col.Update(30, pool, 100, 1.0)
		if len(col.Drops) > 0 {
			break
		}
	}

	if len(col.Drops) == 0 {
		t.Errorf("expected drop to be spawned within 50 updates at 100%% density")
	}
}
