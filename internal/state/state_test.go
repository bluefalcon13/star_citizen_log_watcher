package state

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"StarCitizenLogWatcher/pkg/models"
)

func TestStateManager_StartupSubsumptionFailure(t *testing.T) {
	tempDir := t.TempDir()
	persistPath := filepath.Join(tempDir, "test_latent.json")

	mgr := NewManagerWithPersistence(persistPath)

	guid := "40a7a5ca-59da-11f1-55d5-d3a0356e3f93"

	// Simulate Subsumption error occurring during Application Launch (before game/menu)
	mgr.Record(models.Event{
		Timestamp: "2026-11-08T02:00:05.000Z",
		Type:      models.EventSubsumptionError,
		Severity:  models.SeverityError,
		ErrorGUID: guid,
		RawLine:   "[Error] <[Subsumption::ErrorReporter]> Failed to create variable",
	})

	snap := mgr.Snapshot()
	if !snap.IsStartupSubsumptionBroken {
		t.Fatalf("expected IsStartupSubsumptionBroken = true")
	}
	if snap.StartupSubsumptionErrCount != 1 {
		t.Errorf("StartupSubsumptionErrCount = %d, want 1", snap.StartupSubsumptionErrCount)
	}
	if len(snap.DesyncIncidents) != 1 {
		t.Fatalf("len(DesyncIncidents) = %d, want 1", len(snap.DesyncIncidents))
	}
	if !snap.DesyncIncidents[0].IsStartupError {
		t.Errorf("expected IsStartupError = true")
	}
}

func TestStateManager_ThreeWayContractVerification(t *testing.T) {
	tempDir := t.TempDir()
	persistPath := filepath.Join(tempDir, "test_latent_3way.json")

	mgr1 := NewManagerWithPersistence(persistPath)
	mid := "e0a668ab-90e3-4129-9bf9-0af0ae24ce36"

	// 1. Contract Accepted
	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:00:00.000Z",
		Type:      models.EventMissionAccepted,
		Severity:  models.SeverityInfo,
		MissionID: mid,
		Details:   "Contract Accepted: Bounty Hunt",
	})

	// 2. Contract Status from Generator
	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:25:00.000Z",
		Type:      models.EventContractStatus,
		Severity:  models.SeverityInfo,
		Details:   "Status: completed",
	})

	// 3. Mission Result Validation
	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:26:00.000Z",
		Type:      models.EventMissionResult,
		Severity:  models.SeverityInfo,
		Details:   "Result: Success",
	})

	// 4. Mission Completed Notification
	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:26:05.000Z",
		Type:      models.EventMissionCompleted,
		Severity:  models.SeverityInfo,
		MissionID: mid,
		Details:   "Contract Complete: Bounty Hunt",
	})

	// Session 2: On next launch, startup Subsumption error occurs
	mgr2 := NewManagerWithPersistence(persistPath)
	mgr2.Record(models.Event{
		Timestamp: "2026-11-08T10:00:02.000Z",
		Type:      models.EventSubsumptionMismatch,
		Severity:  models.SeverityError,
		ErrorGUID: mid,
		RawLine:   "Failed to create Subsumption task archetype missing",
	})

	snap := mgr2.Snapshot()
	if len(snap.DesyncIncidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(snap.DesyncIncidents))
	}
	inc := snap.DesyncIncidents[0]
	if !strings.Contains(inc.Description, "CAUSAL LINK PROVEN") {
		t.Errorf("incident description missing causal link proof, got: %s", inc.Description)
	}
	if !strings.Contains(inc.Description, "GenStatus: Status: completed") {
		t.Errorf("incident description missing generator status, got: %s", inc.Description)
	}
	if !strings.Contains(inc.Description, "Result: Result: Success") {
		t.Errorf("incident description missing result outcome, got: %s", inc.Description)
	}
}

func TestStateManager_CrossLaunchCausalLink(t *testing.T) {
	tempDir := t.TempDir()
	persistPath := filepath.Join(tempDir, "test_latent.json")

	// Session 1: Player accepts and completes a mission, then quits
	mgr1 := NewManagerWithPersistence(persistPath)
	mid := "e0a668ab-90e3-4129-9bf9-0af0ae24ce36"

	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:00:00.000Z",
		Type:      models.EventMissionAccepted,
		Severity:  models.SeverityInfo,
		MissionID: mid,
	})
	mgr1.Record(models.Event{
		Timestamp: "2026-11-07T20:30:00.000Z",
		Type:      models.EventMissionCompleted,
		Severity:  models.SeverityInfo,
		MissionID: mid,
	})

	// Session 2: Fresh launch next day. Application boots, Subsumption throws error for that GUID!
	mgr2 := NewManagerWithPersistence(persistPath)
	mgr2.Record(models.Event{
		Timestamp: "2026-11-08T10:00:02.000Z",
		Type:      models.EventSubsumptionMismatch,
		Severity:  models.SeverityError,
		ErrorGUID: mid,
		RawLine:   "Failed to create Subsumption task archetype missing",
	})

	snap := mgr2.Snapshot()
	if !snap.IsStartupSubsumptionBroken {
		t.Fatalf("expected IsStartupSubsumptionBroken = true in Session 2")
	}
	if len(snap.DesyncIncidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(snap.DesyncIncidents))
	}
	inc := snap.DesyncIncidents[0]
	if !inc.IsStartupError {
		t.Errorf("expected startup error flag")
	}
	if inc.HistoricalLink == "" {
		t.Errorf("expected historical link to Session 1 contract, got empty")
	}
}

func TestStateManager_StateTransitions(t *testing.T) {
	mgr := NewManager()

	if mgr.CurrentState() != models.SessionStateAppLaunch {
		t.Errorf("initial state = %v, want AppLaunch", mgr.CurrentState())
	}

	// 1. Enter Menu
	mgr.Record(models.Event{
		Type:    models.EventStateTransition,
		Details: "State Transition: Main Menu / FrontEnd",
	})
	if mgr.CurrentState() != models.SessionStateMenu {
		t.Errorf("state after menu transition = %v, want Menu", mgr.CurrentState())
	}

	// 2. Join Server (Session #1)
	mgr.Record(models.Event{
		Type:    models.EventStateTransition,
		Details: "State Transition: Joining Server (DGS Handshake)",
	})
	if mgr.CurrentState() != models.SessionStateJoiningServer {
		t.Errorf("state = %v, want JoiningServer", mgr.CurrentState())
	}
	if mgr.ServerSession() != 1 {
		t.Errorf("server session = %d, want 1", mgr.ServerSession())
	}

	// 3. Enter Gameplay
	mgr.Record(models.Event{
		Type:    models.EventStateTransition,
		Details: "State Transition: Gameplay (In-World)",
	})
	if mgr.CurrentState() != models.SessionStateGameplay {
		t.Errorf("state = %v, want Gameplay", mgr.CurrentState())
	}

	// 4. Return to Menu
	mgr.Record(models.Event{
		Type:    models.EventServerDisconnect,
		Details: "State Transition: Server Disconnect (Returned to Menu)",
	})
	if mgr.CurrentState() != models.SessionStateMenu {
		t.Errorf("state after disconnect = %v, want Menu", mgr.CurrentState())
	}

	// 5. Join Server #2
	mgr.Record(models.Event{
		Type:    models.EventStateTransition,
		Details: "State Transition: Joining Server (DGS Handshake)",
	})
	if mgr.ServerSession() != 2 {
		t.Errorf("server session = %d, want 2", mgr.ServerSession())
	}
}

func TestStateManager_ConcurrentSafety(t *testing.T) {
	mgr := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.IncrementLineCount()
			mgr.Record(models.Event{
				Timestamp: "2026-10-21T05:00:00.000Z",
				Type:      models.EventContractInit,
				Severity:  models.SeverityNotice,
			})
			_ = mgr.Snapshot()
		}(i)
	}

	wg.Wait()
	snap := mgr.Snapshot()
	if snap.TotalLinesProcessed != 50 {
		t.Errorf("TotalLinesProcessed = %d, want 50", snap.TotalLinesProcessed)
	}
	if snap.EventCounts[models.EventContractInit] != 50 {
		t.Errorf("ContractInit count = %d, want 50", snap.EventCounts[models.EventContractInit])
	}
}
