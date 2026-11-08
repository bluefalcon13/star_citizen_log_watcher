package dashboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"StarCitizenLogWatcher/internal/state"
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

	// Subsumption Startup Health Check
	fmt.Fprintf(d.writer, "Client Subsumption AI    : ")
	if snap.IsStartupSubsumptionBroken {
		fmt.Fprintf(d.writer, "%s[CRITICAL] FAILED ON STARTUP (%d launch error(s))%s\n", ansiRed, snap.StartupSubsumptionErrCount, ansiReset)
		fmt.Fprintf(d.writer, "  %s--> Player was ephemeral: Subsumption failed before joining gameplay due to latent ghost contract.%s\n", ansiYellow, ansiReset)
	} else {
		fmt.Fprintf(d.writer, "%s[NOMINAL] No startup initialization failures detected%s\n", ansiGreen, ansiReset)
	}

	fmt.Fprintf(d.writer, "Subsumption Total Errors : %d\n", snap.SubsumptionErrCount)
	fmt.Fprintf(d.writer, "DGS RPC Failures         : %d\n", snap.DGSFailureCount)
	if snap.LastMissionID != "" {
		fmt.Fprintf(d.writer, "Last Mission ID          : %s [%s]\n", snap.LastMissionID, snap.LastMissionStatus)
	}

	if len(snap.DesyncIncidents) > 0 {
		fmt.Fprintf(d.writer, "\n%s[!] SMOKING GUN: %d Desync / Corruption Incident(s) Detected!%s\n", ansiRed, len(snap.DesyncIncidents), ansiReset)
		for i, inc := range snap.DesyncIncidents {
			startupFlag := ""
			if inc.IsStartupError {
				startupFlag = "[LAUNCH SYMPTOM] "
			}
			fmt.Fprintf(d.writer, "    #%d: %s%s (Target: %s)\n", i+1, startupFlag, inc.Description, inc.MissionID)
			if inc.HistoricalLink != "" {
				fmt.Fprintf(d.writer, "        %s--> %s%s\n", ansiGreen, inc.HistoricalLink, ansiReset)
			}
		}
	} else {
		fmt.Fprintf(d.writer, "%s[+] No Desync Incident Detected.%s\n", ansiGreen, ansiReset)
	}
	fmt.Fprintf(d.writer, "\nFull Forensic Report     : %s\n\n", outputPath)
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

	// Macro Session State
	stateDesc := string(snap.CurrentSessionState)
	if snap.ServerSessionIndex > 0 {
		stateDesc = fmt.Sprintf("%s (Server Session #%d)", snap.CurrentSessionState, snap.ServerSessionIndex)
	}
	fmt.Fprintf(d.writer, "Session Lifecycle      : %s%s%s\n", ansiBold, stateDesc, ansiReset)

	// Primary Subsumption Health
	fmt.Fprintf(d.writer, "Subsumption AI Health  : ")
	if snap.IsStartupSubsumptionBroken {
		fmt.Fprintf(d.writer, "%s[TAINTED - FAILED ON LAUNCH]%s (%d startup errors)\n", ansiRed, ansiReset, snap.StartupSubsumptionErrCount)
	} else if snap.SubsumptionErrCount > 0 {
		fmt.Fprintf(d.writer, "%s[DEGRADED]%s (%d gameplay errors)\n", ansiYellow, ansiReset, snap.SubsumptionErrCount)
	} else {
		fmt.Fprintf(d.writer, "%s[NOMINAL]%s\n", ansiGreen, ansiReset)
	}

	fmt.Fprintf(d.writer, "DGS System Health      : ")
	if snap.DGSFailureCount > 0 {
		fmt.Fprintf(d.writer, "%s%d RPC Failures Detected%s\n", ansiRed, snap.DGSFailureCount, ansiReset)
	} else {
		fmt.Fprintf(d.writer, "%sNOMINAL%s\n", ansiGreen, ansiReset)
	}

	lastMid := snap.LastMissionID
	if lastMid == "" {
		lastMid = "None"
	}
	lastStatus := snap.LastMissionStatus
	if lastStatus == "" {
		lastStatus = "Idle"
	}
	fmt.Fprintf(d.writer, "Contract Tracker       : %s%s%s (Status: %s%s%s)\n\n", ansiBold, lastMid, ansiReset, missionColor, lastStatus, ansiReset)

	if len(snap.DesyncIncidents) > 0 {
		fmt.Fprintf(d.writer, "%s⚠️  SMOKING GUN DETECTED: %d Desync Collision(s)%s\n", ansiRed, len(snap.DesyncIncidents), ansiReset)
		for _, inc := range snap.DesyncIncidents {
			tag := ""
			if inc.IsStartupError {
				tag = "[STARTUP SYMPTOM] "
			}
			fmt.Fprintf(d.writer, "    - [%s] %s%s\n", inc.IncidentAt, tag, inc.Description)
		}
	} else {
		fmt.Fprintf(d.writer, "%sState Sync Status      : Clean / No Latent Corruption Detected%s\n", ansiGreen, ansiReset)
	}

	if snap.LastEvent != nil {
		fmt.Fprintf(d.writer, "\nRecent Event           : %s[%s] %s%s\n", statusColor, snap.LastEvent.Type, snap.LastEvent.Details, ansiReset)
	}
}

func getStatusLabel(flashing bool) string {
	if flashing {
		return "⚡ EVENT DETECTED (FLASH)"
	}
	return "● MONITORING"
}
