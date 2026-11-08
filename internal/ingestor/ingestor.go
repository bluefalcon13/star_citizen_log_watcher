package ingestor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

// Ingestor handles reading and tailing Star Citizen log files.
type Ingestor struct {
	pollInterval time.Duration
}

// NewIngestor returns a configured Ingestor instance.
func NewIngestor(pollInterval time.Duration) *Ingestor {
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	return &Ingestor{
		pollInterval: pollInterval,
	}
}

// StreamFile reads a single file to EOF into outCh without closing outCh.
func (in *Ingestor) StreamFile(ctx context.Context, filePath string, outCh chan<- models.RawLogLine) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lineNum++
		raw := models.RawLogLine{
			LineNumber: lineNum,
			Content:    scanner.Text(),
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case outCh <- raw:
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner encountered error while reading %s: %w", filePath, err)
	}

	return nil
}

// StreamStatic performs a one-shot read of the target file to EOF and closes outCh.
func (in *Ingestor) StreamStatic(ctx context.Context, filePath string, outCh chan<- models.RawLogLine) error {
	defer close(outCh)
	return in.StreamFile(ctx, filePath, outCh)
}

// StreamWatch tails the target log file continuously until ctx is cancelled, closing outCh on exit.
func (in *Ingestor) StreamWatch(ctx context.Context, filePath string, outCh chan<- models.RawLogLine) error {
	defer close(outCh)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for watch ingest: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineNum := 0
	var lastOffset int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err == nil {
			lineNum++
			outCh <- models.RawLogLine{
				LineNumber: lineNum,
				Content:    line,
			}
			continue
		}

		if err == io.EOF {
			// Check if file was truncated or rotated
			stat, statErr := file.Stat()
			if statErr == nil {
				currentOffset, seekErr := file.Seek(0, io.SeekCurrent)
				if seekErr == nil && stat.Size() < currentOffset {
					// File was truncated or reset by the game
					_, _ = file.Seek(0, io.SeekStart)
					reader.Reset(file)
					lineNum = 0
					continue
				}
				lastOffset = currentOffset
				_ = lastOffset
			}

			// Wait for new data or cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(in.pollInterval):
			}
			continue
		}

		return fmt.Errorf("unexpected error reading %s: %w", filePath, err)
	}
}
