package recorder

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/models"
)

// Recorder listens to forensic events, maintains a chronological timeline, and writes reports.
type Recorder struct {
	outputPath string
	stateMgr   *state.Manager
	mu         sync.Mutex
	timeline   []models.Event
}

// NewRecorder constructs a Recorder with the target output path and state manager.
func NewRecorder(outputPath string, stateMgr *state.Manager) *Recorder {
	if outputPath == "" {
		outputPath = "Forensic_Report.md"
	}
	return &Recorder{
		outputPath: outputPath,
		stateMgr:   stateMgr,
		timeline:   nil,
	}
}

// Run consumes events from eventCh until closed or ctx cancelled, recording each event.
func (r *Recorder) Run(ctx context.Context, eventCh <-chan models.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			r.RecordEvent(ev)
		}
	}
}

// RecordEvent appends an event to the internal timeline.
func (r *Recorder) RecordEvent(ev models.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timeline = append(r.timeline, ev)
}

// WriteReport compiles and writes the structured forensic report to disk.
func (r *Recorder) WriteReport() error {
	r.mu.Lock()
	events := make([]models.Event, len(r.timeline))
	copy(events, r.timeline)
	r.mu.Unlock()

	var snap state.Snapshot
	if r.stateMgr != nil {
		snap = r.stateMgr.Snapshot()
	}

	file, err := os.Create(r.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create forensic report at %s: %w", r.outputPath, err)
	}
	defer file.Close()

	return r.RenderReport(file, snap, events)
}

// RenderReport outputs the formatted markdown forensic report to the given writer.
func (r *Recorder) RenderReport(w io.Writer, snap state.Snapshot, events []models.Event) error {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Timestamp == events[j].Timestamp {
			return events[i].ParsedAt.Before(events[j].ParsedAt)
		}
		return events[i].Timestamp < events[j].Timestamp
	})

	fmt.Fprintf(w, "# 🕵️ Star Citizen Forensic Black Box Report\n\n")
	fmt.Fprintf(w, "**Report Generated:** %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(w, "**Total Lines Ingested:** %d\n", snap.TotalLinesProcessed)
	fmt.Fprintf(w, "**Total Filtered Events:** %d\n", len(events))
	fmt.Fprintf(w, "**Final Lifecycle State:** `%s` (Server Sessions: %d)\n\n", snap.CurrentSessionState, snap.ServerSessionIndex)

	// Section 1: Executive Summary
	fmt.Fprintf(w, "## 📊 [Executive Summary]\n\n")
	fmt.Fprintf(w, "| Diagnostic Metric | Value |\n")
	fmt.Fprintf(w, "| :--- | :--- |\n")
	if snap.IsStartupSubsumptionBroken {
		fmt.Fprintf(w, "| **Client Subsumption AI** | ❌ **CRITICAL: BROKEN AT APPLICATION LAUNCH** (%d startup error(s)) |\n", snap.StartupSubsumptionErrCount)
	} else if snap.SubsumptionErrCount > 0 {
		fmt.Fprintf(w, "| **Client Subsumption AI** | ⚠️ **DEGRADED IN GAMEPLAY** (%d error(s)) |\n", snap.SubsumptionErrCount)
	} else {
		fmt.Fprintf(w, "| **Client Subsumption AI** | ✅ **NOMINAL** |\n")
	}
	fmt.Fprintf(w, "| **DGS RPC Health** | %d Failure(s) |\n", snap.DGSFailureCount)
	if snap.LastMissionID != "" {
		fmt.Fprintf(w, "| **Last Contract Tracked** | `%s` [%s] |\n", snap.LastMissionID, snap.LastMissionStatus)
	}
	fmt.Fprintf(w, "\n")

	// Section 2: Desync / Smoking Gun Analysis
	fmt.Fprintf(w, "## 🔍 [Forensic Causality Analysis: The Smoking Gun]\n\n")

	if snap.IsStartupSubsumptionBroken {
		fmt.Fprintf(w, "### 🚨 Primary Finding: Startup Subsumption Degradation\n")
		fmt.Fprintf(w, "The client-side Subsumption AI service threw errors **during Application Launch / startup** before server connection or gameplay.\n\n")
		fmt.Fprintf(w, "> **Forensic Rationale:** The client Subsumption service failed to reconcile account-side contract ledger state upon boot. This is the **primary symptom** of a latent state ghost contract created in the *previous* game session.\n\n")
	}

	if len(snap.DesyncIncidents) == 0 {
		fmt.Fprintf(w, "✅ **No Mission-to-Subsumption Desync Detected.** All accepted missions either concluded cleanly or had no collision with Subsumption/DGS failures.\n\n")
	} else {
		fmt.Fprintf(w, "| Incident # | Phase | Timestamp | Target GUID | Diagnostic Finding |\n")
		fmt.Fprintf(w, "| :--- | :--- | :--- | :--- | :--- |\n")
		for idx, inc := range snap.DesyncIncidents {
			phase := string(inc.SessionState)
			if inc.IsStartupError {
				phase = "**STARTUP**"
			}
			desc := inc.Description
			if inc.HistoricalLink != "" {
				desc = fmt.Sprintf("%s *(%s)*", desc, inc.HistoricalLink)
			}
			fmt.Fprintf(w, "| %d | %s | `%s` | `%s` | %s |\n", idx+1, phase, inc.IncidentAt, inc.MissionID, desc)
		}
		fmt.Fprintf(w, "\n")
	}

	// Section 3: Chronological Timeline
	fmt.Fprintf(w, "## ⏱️ [Chronological Timeline]\n\n")
	if len(events) == 0 {
		fmt.Fprintf(w, "No timeline events recorded.\n")
	} else {
		fmt.Fprintf(w, "| Timestamp | State | Severity | Event Type | Mission ID | Error GUID | Details |\n")
		fmt.Fprintf(w, "| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
		for _, ev := range events {
			mID := ev.MissionID
			if mID == "" {
				mID = "—"
			}
			errGUID := ev.ErrorGUID
			if errGUID == "" {
				errGUID = "—"
			}
			detail := cleanDetail(ev.Details)
			ts := ev.Timestamp
			if ts == "" {
				ts = "N/A"
			}
			stateCol := string(ev.SessionState)
			if stateCol == "" {
				stateCol = "Init"
			}
			fmt.Fprintf(w, "| %s | %s | `%s` | %s | `%s` | `%s` | %s |\n", ts, stateCol, ev.Severity, ev.Type, mID, errGUID, detail)
		}
	}

	fmt.Fprintf(w, "\n---\n*Star Citizen Log Watcher — Forensic Black Box Engine*\n")
	return nil
}

func cleanDetail(msg string) string {
	msg = strings.ReplaceAll(msg, "|", "\\|")
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", "")
	if len(msg) > 120 {
		return msg[:117] + "..."
	}
	return msg
}
