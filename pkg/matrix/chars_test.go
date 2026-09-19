package matrix

import (
	"testing"
)

func TestGetCharPool(t *testing.T) {
	tests := []struct {
		charset  CharSet
		minRunes int
	}{
		{CharSetMatrix, 30},
		{CharSetASCII, 50},
		{CharSetBinary, 2},
		{CharSetHex, 16},
		{CharSet("unknown"), 30},
	}

	for _, tt := range tests {
		pool := GetCharPool(tt.charset)
		if len(pool) < tt.minRunes {
			t.Errorf("GetCharPool(%s) returned %d runes, expected at least %d", tt.charset, len(pool), tt.minRunes)
		}
	}
}

func TestRandomRune(t *testing.T) {
	pool := []rune{'A', 'B', 'C'}
	counts := make(map[rune]int)
	for i := 0; i < 100; i++ {
		r := RandomRune(pool)
		counts[r]++
	}

	for _, r := range pool {
		if counts[r] == 0 {
			t.Errorf("expected rune %c to be selected at least once in 100 draws", r)
		}
	}

	emptyR := RandomRune(nil)
	if emptyR != ' ' {
		t.Errorf("expected space rune for empty pool, got %c", emptyR)
	}
}
