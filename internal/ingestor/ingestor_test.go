package ingestor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

func TestIngestor_StreamStatic(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	content := "line 1\nline 2\nline 3\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ing := NewIngestor(10 * time.Millisecond)
	ch := make(chan models.RawLogLine, 10)

	ctx := context.Background()
	err := ing.StreamStatic(ctx, logPath, ch)
	if err != nil {
		t.Fatalf("StreamStatic failed: %v", err)
	}

	var lines []models.RawLogLine
	for l := range ch {
		lines = append(lines, l)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Content != "line 1" || lines[0].LineNumber != 1 {
		t.Errorf("unexpected first line: %+v", lines[0])
	}
}

func TestIngestor_StreamWatch_ContextCancel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_watch.log")

	if err := os.WriteFile(logPath, []byte("initial line\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ing := NewIngestor(10 * time.Millisecond)
	ch := make(chan models.RawLogLine, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := ing.StreamWatch(ctx, logPath, ch)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("StreamWatch returned unexpected error: %v", err)
	}

	// Drain channel
	count := 0
	for range ch {
		count++
	}
	if count < 1 {
		t.Errorf("expected at least 1 line read before cancellation, got %d", count)
	}
}

