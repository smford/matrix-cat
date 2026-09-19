package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileValid(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	expectedContent := "The Matrix has you...\nFollow the white rabbit.\n"

	if err := os.WriteFile(filePath, []byte(expectedContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	content, err := loadFile(filePath)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("got %q, want %q", string(content), expectedContent)
	}
}

func TestLoadFileDirectoryError(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := loadFile(tmpDir)
	if err == nil {
		t.Fatalf("expected error when loading directory, got nil")
	}

	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("expected 'is a directory' error, got: %v", err)
	}
}

func TestLoadFileNonExistent(t *testing.T) {
	_, err := loadFile("non_existent_file_path_12345.txt")
	if err == nil {
		t.Fatalf("expected error for non-existent file, got nil")
	}
}

func TestStreamFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stream_test.txt")
	testData := "Wake up, Neo...\n"

	if err := os.WriteFile(filePath, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	if err := streamFile(filePath, &buf); err != nil {
		t.Fatalf("streamFile failed: %v", err)
	}

	if buf.String() != testData {
		t.Errorf("streamed output mismatch: got %q, want %q", buf.String(), testData)
	}
}

func TestRunInit(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, ".mcatrc")

	// 1. Initial creation
	var outBuf, errBuf bytes.Buffer
	code := runInit([]string{"-config", targetPath}, &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0 from runInit, got %d. stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(outBuf.String(), "initialized configuration file") {
		t.Errorf("expected 'initialized configuration file' in output, got %q", outBuf.String())
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected target config file to exist: %v", err)
	}

	// 2. Second invocation without --force should warn and not error
	outBuf.Reset()
	errBuf.Reset()
	code = runInit([]string{"-config", targetPath}, &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0 when config already exists, got %d", code)
	}
	if !strings.Contains(outBuf.String(), "already exists") {
		t.Errorf("expected 'already exists' warning in output, got %q", outBuf.String())
	}

	// 3. Invocation with --force should succeed
	outBuf.Reset()
	errBuf.Reset()
	code = runInit([]string{"-config", targetPath, "--force"}, &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --force, got %d", code)
	}
	if !strings.Contains(outBuf.String(), "initialized configuration file") {
		t.Errorf("expected 'initialized configuration file' in output with --force, got %q", outBuf.String())
	}

	// 4. Invocation with -f shorthand
	outBuf.Reset()
	errBuf.Reset()
	code = runInit([]string{"-config", targetPath, "-f"}, &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0 with -f, got %d", code)
	}
}

func TestResolveRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mcatrc")
	configContent := "color = cyan\nfps = 60\nsyntax-theme = dracula\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Case 1: Config file overrides defaults
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := resolveRuntimeConfig(
		fs,
		map[string]bool{},
		configPath,
		30, 50, 1.0, "green", "matrix", true, false, false, 4, true, "monokai", "",
	)
	if err != nil {
		t.Fatalf("resolveRuntimeConfig failed: %v", err)
	}
	if cfg.ThemeName != "cyan" {
		t.Errorf("expected ThemeName 'cyan' from config, got %q", cfg.ThemeName)
	}
	if cfg.FPS != 60 {
		t.Errorf("expected FPS 60 from config, got %d", cfg.FPS)
	}
	if cfg.SyntaxTheme != "dracula" {
		t.Errorf("expected SyntaxTheme 'dracula' from config, got %q", cfg.SyntaxTheme)
	}

	// Case 2: CLI flags override config file
	cfgWithFlags, err := resolveRuntimeConfig(
		fs,
		map[string]bool{
			"color": true,
			"fps":   true,
		},
		configPath,
		120, 50, 1.0, "amber", "matrix", true, false, false, 4, true, "monokai", "",
	)
	if err != nil {
		t.Fatalf("resolveRuntimeConfig with CLI overrides failed: %v", err)
	}
	if cfgWithFlags.ThemeName != "amber" {
		t.Errorf("expected ThemeName 'amber' from CLI override, got %q", cfgWithFlags.ThemeName)
	}
	if cfgWithFlags.FPS != 120 {
		t.Errorf("expected FPS 120 from CLI override, got %d", cfgWithFlags.FPS)
	}
	// Untouched config setting remains intact
	if cfgWithFlags.SyntaxTheme != "dracula" {
		t.Errorf("expected SyntaxTheme 'dracula' from config, got %q", cfgWithFlags.SyntaxTheme)
	}

	// Case 3: Explicit non-existent config file fails
	_, err = resolveRuntimeConfig(
		fs,
		map[string]bool{},
		filepath.Join(tmpDir, "non_existent.rc"),
		30, 50, 1.0, "green", "matrix", true, false, false, 4, true, "monokai", "",
	)
	if err == nil {
		t.Fatalf("expected error for non-existent explicit config file, got nil")
	}
}
