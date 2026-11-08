package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEndToEndPipeline(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "Game.log")
	reportFile := filepath.Join(tempDir, "Forensic_Report.md")

	logContent := strings.Join([]string{
		"<2026-10-21T05:01:00.000Z> CContractGenerator::CContractGenerator",
		"<2026-10-21T05:02:14.116Z> [Notice] <SHUDEvent_OnNotification> Contract Accepted: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]",
		"<2026-10-21T05:02:15.500Z> [Error] <[Subsumption::ErrorReporter]> Failed to create Subsumption variable 'List' (GUID:40a7a5ca-59da-11f1-55d5-d3a0356e3f93) [Team_CoreAI][Subsumption]",
		"<2026-10-21T05:03:00.000Z> !!! Failed to register RPC [RPC.DGS.CommArray.Manage]",
		"<2026-10-21T05:04:00.000Z> [Notice] <CContractGenerator::CContractGenerator> [Missions] (Status: completed)",
		"<2026-10-21T05:04:05.000Z> Contract Complete: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]",
	}, "\n") + "\n"

	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to create test log file: %v", err)
	}

	// Backup os.Args and restore at the end
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{
		"StarCitizenLogWatcher",
		"-file", logFile,
		"-output", reportFile,
	}

	if err := run(); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	reportBytes, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("failed to read generated report: %v", err)
	}

	report := string(reportBytes)
	if !strings.Contains(report, "Star Citizen Forensic Black Box Report") {
		t.Errorf("report missing expected title header")
	}
	if !strings.Contains(report, "The Smoking Gun") {
		t.Errorf("report missing expected smoking gun section")
	}
	if !strings.Contains(report, "40a7a5ca-59da-11f1-55d5-d3a0356e3f93") {
		t.Errorf("report missing expected mission GUID")
	}
}

func TestEndToEndPipeline_Directory(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	file1 := filepath.Join(logDir, "Game_Session1.log")
	file2 := filepath.Join(logDir, "Game_Session2.log")
	reportFile := filepath.Join(tempDir, "Directory_Report.md")

	content1 := "<2026-10-21T05:02:14.116Z> Contract Accepted: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]\n"
	content2 := "<2026-10-21T06:00:00.000Z> Contract Complete: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]\n"

	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // ensure file2 has later ModTime
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{
		"StarCitizenLogWatcher",
		"-dir", logDir,
		"-output", reportFile,
	}

	if err := run(); err != nil {
		t.Fatalf("run() returned error in directory mode: %v", err)
	}

	reportBytes, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("failed to read generated report: %v", err)
	}

	report := string(reportBytes)
	if !strings.Contains(report, "40a7a5ca-59da-11f1-55d5-d3a0356e3f93") {
		t.Errorf("report missing expected mission GUID")
	}
	if !strings.Contains(report, "Total Lines Ingested:** 2") {
		t.Errorf("report missing expected total lines ingested (2 lines across 2 files)")
	}
}
