package matrix

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FPS != 30 {
		t.Errorf("expected FPS 30, got %d", cfg.FPS)
	}
	if cfg.Density != 50 {
		t.Errorf("expected Density 50, got %d", cfg.Density)
	}
	if cfg.SpeedScale != 1.0 {
		t.Errorf("expected SpeedScale 1.0, got %f", cfg.SpeedScale)
	}
	if cfg.ThemeName != "green" {
		t.Errorf("expected ThemeName 'green', got %q", cfg.ThemeName)
	}
	if cfg.CharSetName != "matrix" {
		t.Errorf("expected CharSetName 'matrix', got %q", cfg.CharSetName)
	}
	if !cfg.BoldHead {
		t.Errorf("expected BoldHead to be true")
	}
	if cfg.Center {
		t.Errorf("expected Center to be false")
	}
	if cfg.Loop {
		t.Errorf("expected Loop to be false")
	}
	if cfg.TabWidth != 4 {
		t.Errorf("expected TabWidth 4, got %d", cfg.TabWidth)
	}
	if !cfg.SyntaxHighlight {
		t.Errorf("expected SyntaxHighlight to be true")
	}
	if cfg.SyntaxTheme != "monokai" {
		t.Errorf("expected SyntaxTheme 'monokai', got %q", cfg.SyntaxTheme)
	}
}

func TestParseConfigValid(t *testing.T) {
	input := `
# System configuration
fps = 60
density: 80
speed = 2.5
color = "cyan" # inline comment
charset = 'hex' ; another inline comment
bold = yes
center = true
loop = false
tabwidth = 8
syntax = 1
syntax-theme = dracula
`
	settings, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error parsing config: %v", err)
	}

	if settings["fps"] != "60" {
		t.Errorf("expected fps '60', got %q", settings["fps"])
	}
	if settings["density"] != "80" {
		t.Errorf("expected density '80', got %q", settings["density"])
	}
	if settings["speed"] != "2.5" {
		t.Errorf("expected speed '2.5', got %q", settings["speed"])
	}
	if settings["color"] != "cyan" {
		t.Errorf("expected color 'cyan', got %q", settings["color"])
	}
	if settings["charset"] != "hex" {
		t.Errorf("expected charset 'hex', got %q", settings["charset"])
	}
	if settings["bold"] != "yes" {
		t.Errorf("expected bold 'yes', got %q", settings["bold"])
	}
	if settings["center"] != "true" {
		t.Errorf("expected center 'true', got %q", settings["center"])
	}
	if settings["loop"] != "false" {
		t.Errorf("expected loop 'false', got %q", settings["loop"])
	}
	if settings["tabwidth"] != "8" {
		t.Errorf("expected tabwidth '8', got %q", settings["tabwidth"])
	}
	if settings["syntax"] != "1" {
		t.Errorf("expected syntax '1', got %q", settings["syntax"])
	}
	if settings["syntaxtheme"] != "dracula" {
		t.Errorf("expected syntaxtheme 'dracula', got %q", settings["syntaxtheme"])
	}
}

func TestParseConfigSyntaxErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing separator", "fps 60\n"},
		{"missing key", "= 60\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestApplyConfigMap(t *testing.T) {
	cfg := DefaultConfig()
	settings := map[string]string{
		"fps":         "45",
		"density":     "75",
		"speed":       "1.5",
		"color":       "amber",
		"charset":     "ascii",
		"bold":        "no",
		"center":      "yes",
		"loop":        "1",
		"tabwidth":    "2",
		"syntax":      "off",
		"syntaxtheme": "nord",
	}

	if err := ApplyConfigMap(&cfg, settings); err != nil {
		t.Fatalf("unexpected error applying config: %v", err)
	}

	if cfg.FPS != 45 {
		t.Errorf("expected FPS 45, got %d", cfg.FPS)
	}
	if cfg.Density != 75 {
		t.Errorf("expected Density 75, got %d", cfg.Density)
	}
	if cfg.SpeedScale != 1.5 {
		t.Errorf("expected SpeedScale 1.5, got %f", cfg.SpeedScale)
	}
	if cfg.ThemeName != "amber" {
		t.Errorf("expected ThemeName 'amber', got %q", cfg.ThemeName)
	}
	if cfg.CharSetName != "ascii" {
		t.Errorf("expected CharSetName 'ascii', got %q", cfg.CharSetName)
	}
	if cfg.BoldHead {
		t.Errorf("expected BoldHead to be false")
	}
	if !cfg.Center {
		t.Errorf("expected Center to be true")
	}
	if !cfg.Loop {
		t.Errorf("expected Loop to be true")
	}
	if cfg.TabWidth != 2 {
		t.Errorf("expected TabWidth 2, got %d", cfg.TabWidth)
	}
	if cfg.SyntaxHighlight {
		t.Errorf("expected SyntaxHighlight to be false")
	}
	if cfg.SyntaxTheme != "nord" {
		t.Errorf("expected SyntaxTheme 'nord', got %q", cfg.SyntaxTheme)
	}
}

func TestApplyConfigMapValidationErrors(t *testing.T) {
	invalidCases := []struct {
		key string
		val string
	}{
		{"fps", "5"},        // below min 10
		{"fps", "200"},      // above max 120
		{"fps", "fast"},     // not a number
		{"density", "0"},    // below min 1
		{"density", "101"},  // above max 100
		{"speed", "0.05"},   // below min 0.2
		{"speed", "5.0"},    // above max 3.0
		{"color", "purple"}, // unknown color palette
		{"charset", "utf8"}, // unknown charset
		{"bold", "maybe"},   // invalid boolean
		{"tabwidth", "0"},   // below min 1
		{"tabwidth", "20"},  // above max 16
		{"syntaxtheme", "nonexistent-theme-xyz-123"},
		{"unknownkey", "test"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.key+"="+tc.val, func(t *testing.T) {
			cfg := DefaultConfig()
			err := ApplyConfigMap(&cfg, map[string]string{tc.key: tc.val})
			if err == nil {
				t.Fatalf("expected validation error for %s=%s, got nil", tc.key, tc.val)
			}
		})
	}
}

func TestWriteDefaultConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, ".mcatrc")

	// 1. Initial write should succeed
	if err := WriteDefaultConfigFile(targetPath, false); err != nil {
		t.Fatalf("WriteDefaultConfigFile failed: %v", err)
	}

	// 2. Writing again without force should return ErrConfigFileExists
	err := WriteDefaultConfigFile(targetPath, false)
	if !errors.Is(err, ErrConfigFileExists) {
		t.Fatalf("expected ErrConfigFileExists, got: %v", err)
	}

	// 3. Writing again with force should succeed
	if err := WriteDefaultConfigFile(targetPath, true); err != nil {
		t.Fatalf("WriteDefaultConfigFile with force failed: %v", err)
	}

	// 4. Validate that the generated file parses cleanly and matches DefaultConfig()
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read generated config file: %v", err)
	}
	settings, err := ParseConfig(strings.NewReader(string(content)))
	if err != nil {
		t.Fatalf("failed to parse generated config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := ApplyConfigMap(&cfg, settings); err != nil {
		t.Fatalf("failed to apply generated config: %v", err)
	}

	def := DefaultConfig()
	if cfg.FPS != def.FPS || cfg.Density != def.Density || cfg.SpeedScale != def.SpeedScale ||
		cfg.ThemeName != def.ThemeName || cfg.CharSetName != def.CharSetName ||
		cfg.BoldHead != def.BoldHead || cfg.Center != def.Center || cfg.Loop != def.Loop ||
		cfg.TabWidth != def.TabWidth || cfg.SyntaxHighlight != def.SyntaxHighlight ||
		cfg.SyntaxTheme != def.SyntaxTheme {
		t.Errorf("parsed config from default file does not match DefaultConfig()")
	}
}

func TestLoadAndApplyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "custom.rc")
	content := "color = red\nfps = 40\nspeed = 1.8\n"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := LoadAndApplyConfig(&cfg, filePath); err != nil {
		t.Fatalf("LoadAndApplyConfig failed: %v", err)
	}

	if cfg.ThemeName != "red" {
		t.Errorf("expected ThemeName 'red', got %q", cfg.ThemeName)
	}
	if cfg.FPS != 40 {
		t.Errorf("expected FPS 40, got %d", cfg.FPS)
	}
	if cfg.SpeedScale != 1.8 {
		t.Errorf("expected SpeedScale 1.8, got %f", cfg.SpeedScale)
	}
	if cfg.ConfigFile != filePath {
		t.Errorf("expected ConfigFile %q, got %q", filePath, cfg.ConfigFile)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	p, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath error: %v", err)
	}
	if !strings.HasSuffix(p, ".mcatrc") {
		t.Errorf("expected path ending in .mcatrc, got %q", p)
	}
}
