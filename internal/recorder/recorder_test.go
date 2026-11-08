package recorder

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/models"
)

func TestRecorder_RenderReport(t *testing.T) {
	stateMgr := state.NewManager()
	rec := NewRecorder("dummy.md", stateMgr)

	events := []models.Event{
		{
			Timestamp: "2026-10-21T05:02:14.116Z",
			Type:      models.EventMissionAccepted,
			Severity:  models.SeverityInfo,
			MissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			Details:   "Contract Accepted: Bounty Hunt",
			ParsedAt:  time.Now(),
		},
		{
			Timestamp: "2026-10-21T05:02:15.000Z",
			Type:      models.EventSubsumptionError,
			Severity:  models.SeverityError,
			ErrorGUID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			Details:   "Subsumption variable failure",
			ParsedAt:  time.Now(),
		},
	}

	for _, ev := range events {
		stateMgr.Record(ev)
		rec.RecordEvent(ev)
	}

	var buf bytes.Buffer
	err := rec.RenderReport(&buf, stateMgr.Snapshot(), events)
	if err != nil {
		t.Fatalf("RenderReport failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Forensic Black Box Report") {
		t.Errorf("expected report header in output")
	}
	if !strings.Contains(output, "Smoking Gun") {
		t.Errorf("expected smoking gun section in output")
	}
	if !strings.Contains(output, "40a7a5ca-59da-11f1-55d5-d3a0356e3f93") {
		t.Errorf("expected mission ID in output")
	}
}

