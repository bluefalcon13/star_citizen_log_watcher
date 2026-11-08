package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"StarCitizenLogWatcher/args"
	"StarCitizenLogWatcher/internal/dashboard"
	"StarCitizenLogWatcher/internal/ingestor"
	"StarCitizenLogWatcher/internal/processor"
	"StarCitizenLogWatcher/internal/recorder"
	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/filter"
	"StarCitizenLogWatcher/pkg/models"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Parse and validate CLI arguments
	cfg, err := args.Parse()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("configuration error: %w", err)
	}

	// 2. Set up root context with OS interrupt signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 3. Initialize Core Engine components via Dependency Injection
	registry, err := filter.NewDefaultRegistry()
	if err != nil {
		return fmt.Errorf("filter registry initialization failed: %w", err)
	}

	stateMgr := state.NewManager()
	rec := recorder.NewRecorder(cfg.OutputPath, stateMgr)
	proc := processor.NewProcessor(registry, stateMgr)
	ing := ingestor.NewIngestor(250 * time.Millisecond)
	dash := dashboard.NewDashboard(stateMgr, os.Stdout)

	// 4. Setup decoupled, buffered communication channels
	rawLineCh := make(chan models.RawLogLine, 1000)
	eventCh := make(chan models.Event, 256)

	// 5. Concurrency Management
	var pipelineWg sync.WaitGroup

	// Start Recorder consumer
	pipelineWg.Add(1)
	go func() {
		defer pipelineWg.Done()
		rec.Run(ctx, eventCh)
	}()

	// Start Processor brain
	pipelineWg.Add(1)
	go func() {
		defer pipelineWg.Done()
		proc.Run(ctx, rawLineCh, eventCh)
	}()

	// If in watch mode, launch live TUI observer
	if cfg.WatchMode {
		go dash.RunWatch(ctx)
	}

	// Start Ingestor producer
	var ingestErr error
	if cfg.WatchMode {
		fmt.Printf("[*] Star Citizen Log Watcher: Monitoring %s in Watch Mode (Ctrl+C to quit)...\n", cfg.FilePath)
		ingestErr = ing.StreamWatch(ctx, cfg.FilePath, rawLineCh)
	} else if cfg.Dir != "" {
		ingestErr = ingestDirectory(ctx, ing, cfg.Dir, rawLineCh, cfg.Verbose)
	} else {
		if cfg.Verbose {
			fmt.Printf("[*] Ingesting static log: %s\n", cfg.FilePath)
		}
		ingestErr = ing.StreamStatic(ctx, cfg.FilePath, rawLineCh)
	}

	if ingestErr != nil && ingestErr != context.Canceled {
		cancel()
		pipelineWg.Wait()
		return fmt.Errorf("ingest failure: %w", ingestErr)
	}

	// Wait for Processor and Recorder to drain queues
	pipelineWg.Wait()

	// Cancel dashboard if running
	cancel()

	// 6. Write final forensic report to disk
	if err := rec.WriteReport(); err != nil {
		return fmt.Errorf("failed to write forensic report: %w", err)
	}

	// 7. Render final telemetry summary to terminal
	dash.RenderStatic(stateMgr.Snapshot(), cfg.OutputPath)

	return nil
}

type logFileInfo struct {
	path    string
	modTime time.Time
}

func ingestDirectory(ctx context.Context, ing *ingestor.Ingestor, dir string, outCh chan<- models.RawLogLine, verbose bool) error {
	defer close(outCh)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var logFiles []logFileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		logFiles = append(logFiles, logFileInfo{
			path:    fullPath,
			modTime: info.ModTime(),
		})
	}

	if len(logFiles) == 0 {
		return fmt.Errorf("no .log files found in directory: %s", dir)
	}

	// Sort chronologically by file modification time so Session N precedes Session N+1
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].modTime.Before(logFiles[j].modTime)
	})

	fmt.Printf("[*] Discovered %d .log file(s) in %s (processing chronologically):\n", len(logFiles), dir)

	for idx, lf := range logFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Printf("    [%d/%d] Ingesting %s...\n", idx+1, len(logFiles), filepath.Base(lf.path))
		if err := ing.StreamFile(ctx, lf.path, outCh); err != nil {
			return fmt.Errorf("error processing %s: %w", lf.path, err)
		}
	}

	return nil
}
