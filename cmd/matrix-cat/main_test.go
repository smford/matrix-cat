package main

import (
	"bytes"
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
