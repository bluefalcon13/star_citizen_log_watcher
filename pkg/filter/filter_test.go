package filter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"StarCitizenLogWatcher/pkg/models"
)

func TestDefaultRegistryPatterns(t *testing.T) {
	reg, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry failed: %v", err)
	}

	tests := []struct {
		name                    string
		line                    string
		wantMatch               bool
		wantType                models.EventType
		wantSeverity            models.Severity
		wantMissionID           string
		wantErrorGUID           string
		wantTimestamp           string
		wantActiveInvestigation bool
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
			name:                    "Missing Player ObjectDataBank",
			line:                    `<2026-11-07T05:45:30.558Z> [Error] <CObjectiveMarkerComponent::RemoveFromPlayerDataBank> MissionObjectiveMarker_65157[65157] - Player: Unavailable[0] has no ObjectDataBank [Team_MissionFeatures][Missions]`,
			wantMatch:               true,
			wantType:                models.EventMissingDataBank,
			wantSeverity:            models.SeverityCritical,
			wantTimestamp:           "2026-11-07T05:45:30.558Z",
			wantActiveInvestigation: true,
		},
		{
			name:                    "Proximity Loss Return to Loop",
			line:                    `<2026-11-07T06:38:44.230Z> [Notice] <SHUDEvent_OnNotification> Added notification "New Objective: Return to Storehouse: " [248] to queue. New queue size: 1, MissionId: [7d34cea8-369f-48d4-ad2f-a60cf8bd5c5c], ObjectiveId: [c131b9f7-365b-1c43-9c72-709c6ee182e5] [Team_CoreGameplayFeatures][Missions][Comms]`,
			wantMatch:               true,
			wantType:                models.EventProximityLoss,
			wantSeverity:            models.SeverityCritical,
			wantMissionID:           "7d34cea8-369f-48d4-ad2f-a60cf8bd5c5c",
			wantTimestamp:           "2026-11-07T06:38:44.230Z",
			wantActiveInvestigation: true,
		},
		{
			name:                    "Collector Contract Generator Storm",
			line:                    `<2026-11-07T03:46:17.839Z> [Notice] <GenerateLocationProperty> Generated Locations - variablename: DropoffLocation_BP[Destination], locations: (Wikelo Emporium Kinga Station [3168785171]) contract: TheCollector_Favours_CouncilScrip [Team_MissionFeatures][Missions]`,
			wantMatch:               true,
			wantType:                models.EventLocationStorm,
			wantSeverity:            models.SeverityWarning,
			wantTimestamp:           "2026-11-07T03:46:17.839Z",
			wantActiveInvestigation: false,
		},
		{
			name:                    "Login Identity Metadata Fault",
			line:                    `<2026-10-21T03:00:54.371Z> [Error] <GetSubscriptionKey Failed> failed with error: UNAUTHENTICATED(16): 'No player Id was found in metadata' [Team_GameServices][Social]`,
			wantMatch:               true,
			wantType:                models.EventIdentityFault,
			wantSeverity:            models.SeverityWarning,
			wantTimestamp:           "2026-10-21T03:00:54.371Z",
			wantActiveInvestigation: false,
		},
		{
			name:                    "Hyperion Engineering Wing Objective",
			line:                    `<2026-11-10T04:35:08.727Z> [Notice] <SHUDEvent_OnNotification> Added notification "New Objective: Retrieve One of the Reagent Codes in Engineering Wing: " [17] to queue. New queue size: 3, MissionId: [de9ab375-6c3c-4f83-8f69-2296541982b9], ObjectiveId: [2d495953-2ac2-5fdd-ae5d-61841ab6faa0] [Team_CoreGameplayFeatures][Missions][Comms]`,
			wantMatch:               true,
			wantType:                models.EventHyperionStage,
			wantSeverity:            models.SeverityInfo,
			wantMissionID:           "de9ab375-6c3c-4f83-8f69-2296541982b9",
			wantTimestamp:           "2026-11-10T04:35:08.727Z",
			wantActiveInvestigation: true,
		},
		{
			name:                    "Deferred Explosion RayCast",
			line:                    `<2026-11-10T04:53:17.157Z> RayWorldIntersection: requested by RWI(queued) CDeferredExplosionEffect::RequestRayCast has no grid set, defaulting to global grid`,
			wantMatch:               true,
			wantType:                models.EventDeferredExplosion,
			wantSeverity:            models.SeverityInfo,
			wantTimestamp:           "2026-11-10T04:53:17.157Z",
			wantActiveInvestigation: false,
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
			if ev.Meaning == "" {
				t.Errorf("expected non-empty plain-English Meaning on matched event")
			}
			if ev.ActiveInvestigation != tt.wantActiveInvestigation {
				t.Errorf("ev.ActiveInvestigation = %v, want %v", ev.ActiveInvestigation, tt.wantActiveInvestigation)
			}
		})
	}
}

func TestLoadOrCreateRegistry_CustomConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "filters.json")

	// 1. Auto-creates default filters.json when missing
	reg, err := LoadOrCreateRegistry(cfgPath)
	if err != nil {
		t.Fatalf("LoadOrCreateRegistry failed: %v", err)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected filters.json to be created on disk: %v", err)
	}
	if len(reg.Filters()) == 0 {
		t.Fatalf("expected non-empty default filters")
	}

	// 2. Customize filters.json with a custom plain-English meaning and active_investigation = true
	customJSON := `[
  {
    "name": "Custom Red Flag",
    "enabled": true,
    "event_type": "SubsumptionError",
    "severity": "CRITICAL",
    "pattern": "CustomFault GUID:([0-9a-fA-F-]+)",
    "extract_type": "error_guid",
    "meaning": "Custom Plain-English Explanation of Fault",
    "active_investigation": true
  },
  {
    "name": "Disabled Noise Filter",
    "enabled": false,
    "event_type": "ContractInit",
    "severity": "NOTICE",
    "pattern": "CContractGenerator::CContractGenerator",
    "extract_type": "none",
    "meaning": "Should be ignored when disabled",
    "active_investigation": false
  }
]`
	if err := os.WriteFile(cfgPath, []byte(customJSON), 0644); err != nil {
		t.Fatalf("failed to write custom filters.json: %v", err)
	}

	reg2, err := LoadOrCreateRegistry(cfgPath)
	if err != nil {
		t.Fatalf("LoadOrCreateRegistry custom failed: %v", err)
	}

	if _, matched := reg2.Evaluate("<2026-11-08T00:00:00Z> CContractGenerator::CContractGenerator"); matched {
		t.Errorf("expected disabled filter not to match")
	}

	ev, matched := reg2.Evaluate("<2026-11-08T00:00:01Z> CustomFault GUID:40a7a5ca-59da-11f1-55d5-d3a0356e3f93")
	if !matched {
		t.Fatalf("expected custom enabled filter to match")
	}
	if !ev.ActiveInvestigation {
		t.Errorf("expected ActiveInvestigation = true from custom config")
	}
	if !strings.Contains(ev.Meaning, "Custom Plain-English Explanation of Fault") {
		t.Errorf("unexpected Meaning: %q", ev.Meaning)
	}
}

