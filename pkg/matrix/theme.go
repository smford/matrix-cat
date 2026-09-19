package matrix

import (
	"fmt"
	"strings"
)

// RGB represents an 8-bit truecolor RGB triple.
type RGB struct {
	R, G, B uint8
}

// ANSI returns the 24-bit truecolor escape sequence for foreground color.
func (c RGB) ANSI(bold bool) string {
	if bold {
		return fmt.Sprintf("\x1b[1;38;2;%d;%d;%dm", c.R, c.G, c.B)
	}
	return fmt.Sprintf("\x1b[22;38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// Theme defines the color gradient and head highlight color for rain streams.
type Theme struct {
	Name      string
	HeadColor RGB
	Gradient  []RGB // From brightest to dimmest
}

// Available Themes
var (
	ThemeMatrix = Theme{
		Name:      "green",
		HeadColor: RGB{230, 255, 230}, // Bright glowing white/mint
		Gradient: []RGB{
			{180, 255, 180},
			{85, 255, 85},
			{50, 220, 50},
			{30, 180, 30},
			{20, 140, 20},
			{15, 100, 15},
			{10, 70, 10},
			{5, 45, 5},
			{0, 25, 0},
		},
	}

	ThemeCyan = Theme{
		Name:      "cyan",
		HeadColor: RGB{240, 255, 255},
		Gradient: []RGB{
			{150, 255, 255},
			{0, 240, 240},
			{0, 200, 210},
			{0, 160, 180},
			{0, 120, 145},
			{0, 90, 115},
			{0, 60, 80},
			{0, 40, 55},
			{0, 20, 30},
		},
	}

	ThemeAmber = Theme{
		Name:      "amber",
		HeadColor: RGB{255, 250, 220},
		Gradient: []RGB{
			{255, 220, 120},
			{255, 180, 40},
			{235, 145, 20},
			{200, 115, 10},
			{160, 90, 5},
			{120, 65, 0},
			{85, 45, 0},
			{55, 30, 0},
			{30, 15, 0},
		},
	}

	ThemeRed = Theme{
		Name:      "red",
		HeadColor: RGB{255, 230, 230},
		Gradient: []RGB{
			{255, 140, 140},
			{255, 60, 60},
			{220, 30, 30},
			{180, 15, 15},
			{140, 10, 10},
			{100, 5, 5},
			{70, 0, 0},
			{45, 0, 0},
			{25, 0, 0},
		},
	}

	ThemeMonochrome = Theme{
		Name:      "white",
		HeadColor: RGB{255, 255, 255},
		Gradient: []RGB{
			{220, 220, 220},
			{180, 180, 180},
			{150, 150, 150},
			{120, 120, 120},
			{90, 90, 90},
			{65, 65, 65},
			{45, 45, 45},
			{30, 30, 30},
			{15, 15, 15},
		},
	}

	ThemeRainbow = Theme{
		Name:      "rainbow",
		HeadColor: RGB{255, 255, 255},
		Gradient: []RGB{
			{255, 105, 180}, // Hot Pink
			{148, 0, 211},   // Violet
			{0, 0, 255},     // Blue
			{0, 255, 255},   // Cyan
			{0, 255, 0},     // Green
			{255, 255, 0},   // Yellow
			{255, 127, 0},   // Orange
			{255, 0, 0},     // Red
		},
	}
)

var AllThemes = []Theme{
	ThemeMatrix,
	ThemeCyan,
	ThemeAmber,
	ThemeRed,
	ThemeMonochrome,
	ThemeRainbow,
}

// GetThemeByName returns a theme matching the given name, defaulting to ThemeMatrix.
func GetThemeByName(name string) Theme {
	switch strings.ToLower(name) {
	case "cyan", "blue":
		return ThemeCyan
	case "amber", "yellow", "orange":
		return ThemeAmber
	case "red":
		return ThemeRed
	case "white", "mono", "monochrome":
		return ThemeMonochrome
	case "rainbow":
		return ThemeRainbow
	case "green", "matrix":
		fallthrough
	default:
		return ThemeMatrix
	}
}

// ColorAtPosition calculates the interpolated/stepped color along the trail.
// distFromHead is 0 for the leading character, 1 for the character right behind it, etc.
func (t Theme) ColorAtPosition(distFromHead int, trailLength int, boldHead bool) string {
	if distFromHead == 0 {
		return t.HeadColor.ANSI(boldHead)
	}

	if trailLength <= 1 || len(t.Gradient) == 0 {
		return t.Gradient[0].ANSI(false)
	}

	// Normalize position along trail length to gradient index
	ratio := float64(distFromHead-1) / float64(trailLength-1)
	if ratio >= 1.0 {
		ratio = 0.999
	}
	idx := int(ratio * float64(len(t.Gradient)))
	if idx >= len(t.Gradient) {
		idx = len(t.Gradient) - 1
	}

	return t.Gradient[idx].ANSI(false)
}

// SettledColor returns the ANSI escape sequence for settled text characters.
func (t Theme) SettledColor(col int) string {
	if t.Name == "rainbow" {
		idx := col % len(t.Gradient)
		return t.Gradient[idx].ANSI(false)
	}
	if t.Name == "white" && len(t.Gradient) > 0 {
		return t.Gradient[0].ANSI(false)
	}
	if len(t.Gradient) > 1 {
		return t.Gradient[1].ANSI(false)
	}
	if len(t.Gradient) > 0 {
		return t.Gradient[0].ANSI(false)
	}
	return t.HeadColor.ANSI(false)
}
