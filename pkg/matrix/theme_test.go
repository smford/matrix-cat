package matrix

import (
	"strings"
	"testing"
)

func TestGetThemeByName(t *testing.T) {
	tests := []struct {
		name         string
		expectedName string
	}{
		{"green", "green"},
		{"matrix", "green"},
		{"cyan", "cyan"},
		{"blue", "cyan"},
		{"amber", "amber"},
		{"red", "red"},
		{"white", "white"},
		{"rainbow", "rainbow"},
		{"non-existent", "green"},
	}

	for _, tt := range tests {
		th := GetThemeByName(tt.name)
		if th.Name != tt.expectedName {
			t.Errorf("GetThemeByName(%q) = %s; want %s", tt.name, th.Name, tt.expectedName)
		}
	}
}

func TestColorAtPosition(t *testing.T) {
	th := ThemeMatrix

	headColor := th.ColorAtPosition(0, 10, true)
	if !strings.Contains(headColor, "38;2;230;255;230m") || !strings.Contains(headColor, "\x1b[1;") {
		t.Errorf("Head color unexpected: %q", headColor)
	}

	trailStart := th.ColorAtPosition(1, 10, false)
	if !strings.Contains(trailStart, "38;2;") {
		t.Errorf("Trail color unexpected: %q", trailStart)
	}

	trailEnd := th.ColorAtPosition(9, 10, false)
	if !strings.Contains(trailEnd, "38;2;") {
		t.Errorf("Tail color unexpected: %q", trailEnd)
	}
}
