package dashboard

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/models"
)

func TestDashboard_RenderStatic(t *testing.T) {
	stateMgr := state.NewManager()
	var buf bytes.Buffer
	dash := NewDashboard(stateMgr, &buf)

	stateMgr.Record(models.Event{
		Timestamp: "2026-10-21T05:02:14.116Z",
		Type:      models.EventMissionAccepted,
		Severity:  models.SeverityInfo,
		MissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
		Details:   "Accepted test mission",
		ParsedAt:  time.Now(),
	})

	dash.RenderStatic(stateMgr.Snapshot(), "test_report.md")

	out := buf.String()
	if !strings.Contains(out, "FORENSIC TELEMETRY COMPLETE") {
		t.Errorf("expected header in static output, got: %s", out)
	}
	if !strings.Contains(out, "40a7a5ca-59da-11f1-55d5-d3a0356e3f93") {
		t.Errorf("expected mission ID in static output, got: %s", out)
	}
}

func TestDashboard_RenderLiveFrame(t *testing.T) {
	stateMgr := state.NewManager()
	var buf bytes.Buffer
	dash := NewDashboard(stateMgr, &buf)

	dash.renderLiveFrame(stateMgr.Snapshot())
	out := buf.String()
	if !strings.Contains(out, "FORENSIC BLACK BOX FLIGHT RECORDER") {
		t.Errorf("expected dashboard frame header, got: %s", out)
	}
}

