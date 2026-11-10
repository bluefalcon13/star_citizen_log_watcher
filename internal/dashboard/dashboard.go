package dashboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/models"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiYellow = "\033[33;1m"
	ansiRed    = "\033[31;1m"
	ansiGreen  = "\033[32;1m"
	ansiCyan   = "\033[36;1m"
	ansiClear  = "\033[H\033[2J"
)

// Dashboard renders real-time forensic telemetry to the terminal.
type Dashboard struct {
	stateMgr      *state.Manager
	writer        io.Writer
	flashDuration time.Duration
	refreshRate   time.Duration
}

// NewDashboard creates an initialized Dashboard observer.
func NewDashboard(stateMgr *state.Manager, writer io.Writer) *Dashboard {
	if writer == nil {
		writer = os.Stdout
	}
	return &Dashboard{
		stateMgr:      stateMgr,
		writer:        writer,
		flashDuration: 2 * time.Second,
		refreshRate:   250 * time.Millisecond,
	}
}

// RenderStatic prints a formatted final summary to the terminal.
func (d *Dashboard) RenderStatic(snap state.Snapshot, outputPath string) {
	fmt.Fprintf(d.writer, "\n%s=== STAR CITIZEN LOG WATCHER: FORENSIC TELEMETRY COMPLETE ===%s\n", ansiCyan, ansiReset)
	fmt.Fprintf(d.writer, "Total Lines Processed    : %d\n", snap.TotalLinesProcessed)
	fmt.Fprintf(d.writer, "Final Session State      : %s (Server Sessions: %d)\n", snap.CurrentSessionState, snap.ServerSessionIndex)
	fmt.Fprintf(d.writer, "Subsumption Log Entries  : %d\n", snap.SubsumptionErrCount)
	fmt.Fprintf(d.writer, "DGS RPC Notices          : %d\n", snap.DGSFailureCount)
	if snap.LastMissionID != "" {
		fmt.Fprintf(d.writer, "Last Mission ID          : %s [%s]\n", snap.LastMissionID, snap.LastMissionStatus)
	}

	if len(snap.ContractLedger) > 0 {
		d.renderContractTable(snap)
	}

	if len(snap.StateChanges) > 0 {
		fmt.Fprintf(d.writer, "\n%s--- Detected State & Contract Changes (Recent) ---%s\n", ansiCyan, ansiReset)
		for _, ev := range snap.StateChanges {
			ts := formatTimestampShort(ev.Timestamp)
			fmt.Fprintf(d.writer, "  [%s] (%s) %s\n", ts, ev.SessionState, ev.Meaning)
		}
	}

	if len(snap.RecentEvents) > 0 {
		fmt.Fprintf(d.writer, "\n%s--- Recent Log Entries (Plain English) ---%s\n", ansiCyan, ansiReset)
		for _, ev := range snap.RecentEvents {
			ts := formatTimestampShort(ev.Timestamp)
			fmt.Fprintf(d.writer, "  [%s] [%s] %s\n", ts, ev.Type, ev.Meaning)
		}
	}

	// Only display Active Investigation alerts if enabled in filters.json and triggered
	if len(snap.DesyncIncidents) > 0 {
		fmt.Fprintf(d.writer, "\n%s[!] ACTIVE INVESTIGATION INCIDENTS: %d Alert(s) Detected!%s\n", ansiRed, len(snap.DesyncIncidents), ansiReset)
		for i, inc := range snap.DesyncIncidents {
			fmt.Fprintf(d.writer, "    #%d: %s (Target: %s)\n", i+1, inc.Description, inc.MissionID)
			if inc.HistoricalLink != "" {
				fmt.Fprintf(d.writer, "        %s--> %s%s\n", ansiGreen, inc.HistoricalLink, ansiReset)
			}
		}
	}

	fmt.Fprintf(d.writer, "\nFull Forensic Report     : %s\n\n", outputPath)
}

// RenderContracts prints a focused summary of all accepted contracts and their outcomes to the terminal.
func (d *Dashboard) RenderContracts(snap state.Snapshot, outputPath string) {
	fmt.Fprintf(d.writer, "\n%s=== STAR CITIZEN CONTRACT LEDGER SCRAPER ===%s\n", ansiCyan, ansiReset)
	fmt.Fprintf(d.writer, "Total Lines Processed    : %d\n", snap.TotalLinesProcessed)
	fmt.Fprintf(d.writer, "Total Contracts Accepted : %d\n", len(snap.ContractLedger))

	if len(snap.ContractLedger) == 0 {
		fmt.Fprintf(d.writer, "\n  No accepted contracts found in the scanned log(s).\n")
	} else {
		d.renderContractTable(snap)
	}

	if len(snap.DesyncIncidents) > 0 {
		fmt.Fprintf(d.writer, "\n%s[!] ACTIVE INVESTIGATION INCIDENTS: %d Alert(s) Detected!%s\n", ansiRed, len(snap.DesyncIncidents), ansiReset)
		for i, inc := range snap.DesyncIncidents {
			fmt.Fprintf(d.writer, "    #%d: %s (Target: %s)\n", i+1, inc.Description, inc.MissionID)
		}
	}

	fmt.Fprintf(d.writer, "\nFull Forensic Report     : %s\n\n", outputPath)
}

func (d *Dashboard) renderContractTable(snap state.Snapshot) {
	fmt.Fprintf(d.writer, "\n%s--- Accepted Contracts & Outcomes (%d) ---%s\n", ansiCyan, len(snap.ContractLedger), ansiReset)
	for i, c := range snap.ContractLedger {
		resColor := ansiYellow
		if c.Result == "Completed" {
			resColor = ansiGreen
		} else if strings.Contains(c.Result, "Failed") || strings.Contains(c.Result, "Stalled") {
			resColor = ansiRed
		}

		logLabel := c.LogBanner
		if logLabel == "" {
			logLabel = fmt.Sprintf("Session #%d", c.ServerSess)
		}
		bpSuffix := ""
		if c.Blueprint != "" {
			bpSuffix = fmt.Sprintf(" {Template: %s}", c.Blueprint)
		}
		anomSuffix := ""
		if len(c.Anomalies) > 0 {
			anomSuffix = fmt.Sprintf(" %s[Anomalies: %s]%s", ansiRed, strings.Join(c.Anomalies, "; "), ansiReset)
		}

		fmt.Fprintf(d.writer, "  #%d [%s] (%s) %s%s%s -> %s%s%s [ID: %s]%s%s\n",
			i+1,
			logLabel,
			formatTimestampShort(c.AcceptedAt),
			ansiBold, c.ContractName, ansiReset,
			resColor, c.Result, ansiReset,
			c.MissionID,
			bpSuffix,
			anomSuffix,
		)
	}
}

// RunWatch begins a live dashboard rendering loop until ctx is cancelled.
func (d *Dashboard) RunWatch(ctx context.Context) {
	ticker := time.NewTicker(d.refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := d.stateMgr.Snapshot()
			d.renderLiveFrame(snap)
		}
	}
}

func (d *Dashboard) renderLiveFrame(snap state.Snapshot) {
	isFlashing := time.Since(snap.RecentEventAt) < d.flashDuration

	statusColor := ansiReset
	if isFlashing {
		statusColor = ansiYellow
	}

	missionColor := ansiCyan
	if snap.LastMissionStatus == "Completed" {
		missionColor = ansiGreen
	} else if snap.LastMissionStatus == "Failed" || snap.LastMissionStatus == "Abandoned" {
		missionColor = ansiRed
	}

	fmt.Fprint(d.writer, ansiClear)
	fmt.Fprintf(d.writer, "%s╔══════════════════════════════════════════════════════════════════════╗%s\n", ansiCyan, ansiReset)
	fmt.Fprintf(d.writer, "%s║          STAR CITIZEN FORENSIC BLACK BOX FLIGHT RECORDER             ║%s\n", ansiCyan, ansiReset)
	fmt.Fprintf(d.writer, "%s╚══════════════════════════════════════════════════════════════════════╝%s\n\n", ansiCyan, ansiReset)

	fmt.Fprintf(d.writer, "Lines Ingested         : %d\n", snap.TotalLinesProcessed)
	fmt.Fprintf(d.writer, "Watcher Status         : %s%s%s\n", statusColor, getStatusLabel(isFlashing), ansiReset)

	stateDesc := string(snap.CurrentSessionState)
	if snap.ServerSessionIndex > 0 {
		stateDesc = fmt.Sprintf("%s (Server Session #%d)", snap.CurrentSessionState, snap.ServerSessionIndex)
	}
	fmt.Fprintf(d.writer, "Current Session State  : %s%s%s\n", ansiBold, stateDesc, ansiReset)

	lastMid := snap.LastMissionID
	if lastMid == "" {
		lastMid = "None"
	}
	lastStatus := snap.LastMissionStatus
	if lastStatus == "" {
		lastStatus = "Idle"
	}
	fmt.Fprintf(d.writer, "Contract Tracker       : %s%s%s (Status: %s%s%s)\n", ansiBold, lastMid, ansiReset, missionColor, lastStatus, ansiReset)
	fmt.Fprintf(d.writer, "Event Counts           : %s\n", formatEventCounts(snap.EventCounts))

	// Section 1: Detected State & Contract Transitions
	fmt.Fprintf(d.writer, "\n%s── Detected State & Contract Changes ──────────────────────────────────%s\n", ansiCyan, ansiReset)
	if len(snap.StateChanges) == 0 {
		fmt.Fprintf(d.writer, "  Waiting for session or contract state changes...\n")
	} else {
		for _, ev := range snap.StateChanges {
			ts := formatTimestampShort(ev.Timestamp)
			color := ansiReset
			if ev.Type == models.EventMissionAccepted || ev.Type == models.EventMissionCompleted {
				color = ansiGreen
			} else if ev.Type == models.EventMissionFailed || ev.Type == models.EventMissionAbandoned {
				color = ansiYellow
			}
			fmt.Fprintf(d.writer, "  %s[%s] (%s) %s%s\n", color, ts, ev.SessionState, ev.Meaning, ansiReset)
		}
	}

	// Section 2: Recent Log Entries in Plain English
	fmt.Fprintf(d.writer, "\n%s── Recent Log Entries (Plain English) ─────────────────────────────────%s\n", ansiCyan, ansiReset)
	if len(snap.RecentEvents) == 0 {
		fmt.Fprintf(d.writer, "  No matching log entries detected yet...\n")
	} else {
		for idx, ev := range snap.RecentEvents {
			ts := formatTimestampShort(ev.Timestamp)
			lineColor := ansiReset
			if idx == len(snap.RecentEvents)-1 && isFlashing {
				lineColor = ansiYellow
			}
			fmt.Fprintf(d.writer, "  %s[%s] [%s] %s%s\n", lineColor, ts, ev.Type, ev.Meaning, ansiReset)
		}
	}

	// Optional Section 3: Active Investigation Alerts (only shown if triggered by an enabled active_investigation filter)
	if len(snap.DesyncIncidents) > 0 {
		fmt.Fprintf(d.writer, "\n%s⚠️  ACTIVE INVESTIGATION INCIDENTS (%d)%s\n", ansiRed, len(snap.DesyncIncidents), ansiReset)
		for _, inc := range snap.DesyncIncidents {
			fmt.Fprintf(d.writer, "    - [%s] %s\n", inc.IncidentAt, inc.Description)
		}
	}
}

func formatTimestampShort(ts string) string {
	if ts == "" {
		return "N/A"
	}
	// If ISO-8601 like 2026-11-08T19:17:32.570Z, keep time portion for compact TUI readability
	if len(ts) >= 19 && ts[10] == 'T' {
		return ts[11:]
	}
	return ts
}

func formatEventCounts(counts map[models.EventType]int) string {
	if len(counts) == 0 {
		return "None"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	out := ""
	for i, k := range keys {
		if i > 0 {
			out += " | "
		}
		out += fmt.Sprintf("%s: %d", k, counts[models.EventType(k)])
	}
	return out
}

func getStatusLabel(flashing bool) string {
	if flashing {
		return "⚡ EVENT DETECTED (FLASH)"
	}
	return "● MONITORING"
}
