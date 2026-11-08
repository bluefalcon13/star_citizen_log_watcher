package args

import (
	"flag"
	"fmt"
	"os"
)

// Args holds validated command-line configuration for the application.
type Args struct {
	FilePath   string
	Dir        string
	WatchMode  bool
	OutputPath string
	Verbose    bool
}

// Parse parses and validates command-line flags according to the implementation plan.
func Parse() (*Args, error) {
	return ParseArgs(os.Args[1:])
}

// ParseArgs parses arguments from an explicit string slice to facilitate automated unit testing.
func ParseArgs(arguments []string) (*Args, error) {
	fs := flag.NewFlagSet("StarCitizenLogWatcher", flag.ContinueOnError)

	filePath := fs.String("file", "", "Path to the target Star Citizen log file (e.g. Game.log)")
	legacyInput := fs.String("input", "", "Legacy alias for -file")
	dirPath := fs.String("dir", "", "Directory containing .log files to batch process chronologically")
	watchMode := fs.Bool("watch", false, "Enable live watch/tail mode for active game sessions")
	outputPath := fs.String("output", "Forensic_Report.md", "Output path for the generated forensic report")
	verbose := fs.Bool("verbose", false, "Enable verbose diagnostic output")

	if err := fs.Parse(arguments); err != nil {
		if err == flag.ErrHelp {
			return nil, err
		}
		return nil, fmt.Errorf("argument parsing error: %w", err)
	}

	targetDir := *dirPath
	targetFile := *filePath
	if targetFile == "" {
		targetFile = *legacyInput
	}

	if targetFile == "" && targetDir == "" {
		return nil, fmt.Errorf("missing input: specify -file <path> or -dir <folder>")
	}

	// If user provided targetFile, check if it's actually a directory
	if targetFile != "" {
		stat, err := os.Stat(targetFile)
		if err != nil {
			if !*watchMode {
				return nil, fmt.Errorf("target file cannot be accessed: %w", err)
			}
		} else if stat.IsDir() {
			// Auto-convert directory passed to -file into Dir mode
			targetDir = targetFile
			targetFile = ""
		}
	}

	// If Dir mode, validate directory and ensure watch mode is not used
	if targetDir != "" {
		stat, err := os.Stat(targetDir)
		if err != nil {
			return nil, fmt.Errorf("target directory cannot be accessed: %w", err)
		}
		if !stat.IsDir() {
			return nil, fmt.Errorf("specified -dir is not a directory: %s", targetDir)
		}
		if *watchMode {
			return nil, fmt.Errorf("-watch mode is not supported with -dir: watch mode requires a specific log file")
		}
	}

	return &Args{
		FilePath:   targetFile,
		Dir:        targetDir,
		WatchMode:  *watchMode,
		OutputPath: *outputPath,
		Verbose:    *verbose,
	}, nil
}
