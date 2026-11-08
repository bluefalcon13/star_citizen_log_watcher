package args

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tempDir := t.TempDir()
	dummyFile := filepath.Join(tempDir, "Game.log")
	if err := os.WriteFile(dummyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	t.Run("Valid static file", func(t *testing.T) {
		a, err := ParseArgs([]string{"-file", dummyFile, "-output", "out.md", "-verbose"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.FilePath != dummyFile {
			t.Errorf("FilePath = %q, want %q", a.FilePath, dummyFile)
		}
		if a.OutputPath != "out.md" {
			t.Errorf("OutputPath = %q, want out.md", a.OutputPath)
		}
		if !a.Verbose {
			t.Errorf("Verbose = false, want true")
		}
		if a.WatchMode {
			t.Errorf("WatchMode = true, want false")
		}
	})

	t.Run("Valid legacy -input alias", func(t *testing.T) {
		a, err := ParseArgs([]string{"-input", dummyFile})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.FilePath != dummyFile {
			t.Errorf("FilePath = %q, want %q", a.FilePath, dummyFile)
		}
	})

	t.Run("Explicit directory flag -dir", func(t *testing.T) {
		a, err := ParseArgs([]string{"-dir", tempDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Dir != tempDir {
			t.Errorf("Dir = %q, want %q", a.Dir, tempDir)
		}
	})

	t.Run("Auto-convert folder passed to -file", func(t *testing.T) {
		a, err := ParseArgs([]string{"-file", tempDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Dir != tempDir {
			t.Errorf("Dir = %q, want %q", a.Dir, tempDir)
		}
		if a.FilePath != "" {
			t.Errorf("FilePath should be empty in Dir mode, got %q", a.FilePath)
		}
	})

	t.Run("Missing path", func(t *testing.T) {
		_, err := ParseArgs([]string{"-verbose"})
		if err == nil {
			t.Errorf("expected error for missing file path, got nil")
		}
	})
}
