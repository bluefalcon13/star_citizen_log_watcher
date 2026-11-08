package filter

import (
	"testing"

	"StarCitizenLogWatcher/pkg/models"
)

func TestDefaultRegistryPatterns(t *testing.T) {
	reg, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry failed: %v", err)
	}

	tests := []struct {
		name          string
		line          string
		wantMatch     bool
		wantType      models.EventType
		wantSeverity  models.Severity
		wantMissionID string
		wantErrorGUID string
		wantTimestamp string
	}{
		{
			name:          "Mission Accepted",
			line:          `<2026-10-21T05:02:14.116Z> [Notice] <SHUDEvent_OnNotification> Contract Accepted: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]`,
			wantMatch:     true,
			wantType:      models.EventMissionAccepted,
			wantSeverity:  models.SeverityInfo,
			wantMissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			wantTimestamp: "2026-10-21T05:02:14.116Z",
		},
		{
			name:          "Mission Completed",
			line:          `<2026-10-21T05:22:10.010Z> [Notice] <SHUDEvent_OnNotification> Contract Complete: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]`,
			wantMatch:     true,
			wantType:      models.EventMissionCompleted,
			wantSeverity:  models.SeverityInfo,
			wantMissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			wantTimestamp: "2026-10-21T05:22:10.010Z",
		},
		{
			name:          "Mission Failed",
			line:          `<2026-10-21T05:22:10.010Z> [Notice] <SHUDEvent_OnNotification> Contract Failed: Bounty Hunt MissionId: [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]`,
			wantMatch:     true,
			wantType:      models.EventMissionFailed,
			wantSeverity:  models.SeverityWarning,
			wantMissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			wantTimestamp: "2026-10-21T05:22:10.010Z",
		},
		{
			name:          "Mission Abandoned",
			line:          `<2026-10-21T05:22:10.010Z> [Notice] <SHUDEvent_OnNotification> Contract Abandoned: Delivery MissionId: [e0a668ab-90e3-4129-9bf9-0af0ae24ce36]`,
			wantMatch:     true,
			wantType:      models.EventMissionAbandoned,
			wantSeverity:  models.SeverityWarning,
			wantMissionID: "e0a668ab-90e3-4129-9bf9-0af0ae24ce36",
			wantTimestamp: "2026-10-21T05:22:10.010Z",
		},
		{
			name:          "Objective Spawned",
			line:          `<2026-10-21T05:03:00.000Z> CLocalMissionPhaseMarker::CreateMarker with markerId [123] missionId [40a7a5ca-59da-11f1-55d5-d3a0356e3f93]`,
			wantMatch:     true,
			wantType:      models.EventObjectiveSpawned,
			wantSeverity:  models.SeverityInfo,
			wantMissionID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			wantTimestamp: "2026-10-21T05:03:00.000Z",
		},
		{
			name:          "DGS Failure",
			line:          `<2026-10-20T08:08:32.264Z> !!! Failed to register RPC [RPC.DGS.CommArray.Manage]`,
			wantMatch:     true,
			wantType:      models.EventDGSFailure,
			wantSeverity:  models.SeverityCritical,
			wantTimestamp: "2026-10-20T08:08:32.264Z",
		},
		{
			name:          "Subsumption Error",
			line:          `<2026-10-18T13:54:04.860Z> [Error] <[Subsumption::ErrorReporter]> List File: '...' - Failed to create Subsumption variable 'List' (GUID:40a7a5ca-59da-11f1-55d5-d3a0356e3f93) ... [Team_CoreAI][Subsumption]`,
			wantMatch:     true,
			wantType:      models.EventSubsumptionError,
			wantSeverity:  models.SeverityError,
			wantErrorGUID: "40a7a5ca-59da-11f1-55d5-d3a0356e3f93",
			wantTimestamp: "2026-10-18T13:54:04.860Z",
		},
		{
			name:          "Subsumption Mismatch",
			line:          `<2026-10-18T13:54:04.908Z> Failed to create Subsumption task because the Archetype associated with the guid 'e0a668ab-90e3-4129-9bf9-0af0ae24ce36' is not present`,
			wantMatch:     true,
			wantType:      models.EventSubsumptionMismatch,
			wantSeverity:  models.SeverityError,
			wantErrorGUID: "e0a668ab-90e3-4129-9bf9-0af0ae24ce36",
			wantTimestamp: "2026-10-18T13:54:04.908Z",
		},
		{
			name:          "Contract Status",
			line:          `<2026-10-18T13:54:01.462Z> [Notice] <CContractGenerator::CContractGenerator> [Missions] (Status: completed)`,
			wantMatch:     true,
			wantType:      models.EventContractStatus,
			wantSeverity:  models.SeverityInfo,
			wantTimestamp: "2026-10-18T13:54:01.462Z",
		},
		{
			name:          "Mission Result",
			line:          `<2026-10-18T13:55:00.000Z> Mission outcome is Success`,
			wantMatch:     true,
			wantType:      models.EventMissionResult,
			wantSeverity:  models.SeverityInfo,
			wantTimestamp: "2026-10-18T13:55:00.000Z",
		},
		{
			name:      "Unrelated log line",
			line:      `<2026-10-18T13:55:00.000Z> Loading graphics shaders 100% complete`,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, matched := reg.Evaluate(tt.line)
			if matched != tt.wantMatch {
				t.Fatalf("Evaluate() matched = %v, want %v", matched, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if ev.Type != tt.wantType {
				t.Errorf("ev.Type = %v, want %v", ev.Type, tt.wantType)
			}
			if ev.Severity != tt.wantSeverity {
				t.Errorf("ev.Severity = %v, want %v", ev.Severity, tt.wantSeverity)
			}
			if tt.wantMissionID != "" && ev.MissionID != tt.wantMissionID {
				t.Errorf("ev.MissionID = %q, want %q", ev.MissionID, tt.wantMissionID)
			}
			if tt.wantErrorGUID != "" && ev.ErrorGUID != tt.wantErrorGUID {
				t.Errorf("ev.ErrorGUID = %q, want %q", ev.ErrorGUID, tt.wantErrorGUID)
			}
			if tt.wantTimestamp != "" && ev.Timestamp != tt.wantTimestamp {
				t.Errorf("ev.Timestamp = %q, want %q", ev.Timestamp, tt.wantTimestamp)
			}
		})
	}
}

