package processor

import (
	"context"
	"testing"

	"StarCitizenLogWatcher/internal/state"
	"StarCitizenLogWatcher/pkg/filter"
	"StarCitizenLogWatcher/pkg/models"
)

func TestProcessor_Run(t *testing.T) {
	reg, err := filter.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry failed: %v", err)
	}

	stateMgr := state.NewManager()
	proc := NewProcessor(reg, stateMgr)

	rawCh := make(chan models.RawLogLine, 5)
	eventCh := make(chan models.Event, 5)

	rawCh <- models.RawLogLine{
		LineNumber: 1,
		Content:    "<2026-10-21T05:02:14.116Z> Contract Accepted: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]",
	}
	rawCh <- models.RawLogLine{
		LineNumber: 2,
		Content:    "<2026-10-21T05:02:15.000Z> Regular debug message that should not match",
	}
	close(rawCh)

	ctx := context.Background()
	proc.Run(ctx, rawCh, eventCh)

	var events []models.Event
	for ev := range eventCh {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != models.EventMissionAccepted {
		t.Errorf("expected EventMissionAccepted, got %v", events[0].Type)
	}

	snap := stateMgr.Snapshot()
	if snap.TotalLinesProcessed != 2 {
		t.Errorf("TotalLinesProcessed = %d, want 2", snap.TotalLinesProcessed)
	}
	if snap.EventCounts[models.EventMissionAccepted] != 1 {
		t.Errorf("EventMissionAccepted count = %d, want 1", snap.EventCounts[models.EventMissionAccepted])
	}
}

