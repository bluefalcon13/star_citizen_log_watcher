package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

// DesyncIncident records a detected collision between an active mission and a Subsumption/DGS failure.
type DesyncIncident struct {
	MissionID      string
	IncidentAt     string
	Description    string
	TriggerLine    string
	IsStartupError bool
	HistoricalLink string
	SessionState   models.SessionState
	ServerSession  int
}

// Snapshot provides a point-in-time immutable view of the log watcher telemetry.
type Snapshot struct {
	TotalLinesProcessed        int
	EventCounts                map[models.EventType]int
	SeverityCounts             map[models.Severity]int
	LastEvent                  *models.Event
	LastMissionID              string
	LastMissionStatus          string
	DGSFailureCount            int
	SubsumptionErrCount        int
	StartupSubsumptionErrCount int
	IsStartupSubsumptionBroken bool
	CurrentSessionState        models.SessionState
	ServerSessionIndex         int
	DesyncIncidents            []DesyncIncident
	RecentEventAt              time.Time
}

// Manager maintains thread-safe live telemetry, macro session states, and causal state for Star Citizen.
type Manager struct {
	mu                         sync.RWMutex
	totalLinesProcessed        int
	eventCounts                map[models.EventType]int
	severityCounts             map[models.Severity]int
	lastEvent                  *models.Event
	lastMissionID              string
	lastMissionStatus          string
	activeMissions             map[string]bool
	objectiveSpawned           map[string]bool
	dgsFailureCount            int
	subsumptionErrCount        int
	startupSubsumptionErrCount int
	isStartupSubsumptionBroken bool
	currentSessionState        models.SessionState
	serverSessionIndex         int
	desyncIncidents            []DesyncIncident
	recentEventAt              time.Time
	historicalContracts        map[string]models.HistoricalContract
	persistentFilePath         string
}

// NewManager creates a thread-safe StateManager instance with default AppLaunch state.
func NewManager() *Manager {
	return NewManagerWithPersistence(".sc_latent_state.json")
}

// NewManagerWithPersistence creates an initialized StateManager with custom persistence path.
func NewManagerWithPersistence(persistPath string) *Manager {
	m := &Manager{
		eventCounts:         make(map[models.EventType]int),
		severityCounts:      make(map[models.Severity]int),
		activeMissions:      make(map[string]bool),
		objectiveSpawned:    make(map[string]bool),
		currentSessionState: models.SessionStateAppLaunch,
		serverSessionIndex:  0,
		historicalContracts: make(map[string]models.HistoricalContract),
		persistentFilePath:  persistPath,
		desyncIncidents:     nil,
	}

	if persistPath != "" {
		_ = m.loadPersistentState(persistPath)
	}

	return m
}

// IncrementLineCount updates the total lines ingested.
func (m *Manager) IncrementLineCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalLinesProcessed++
}

// CurrentState returns the current macro session state.
func (m *Manager) CurrentState() models.SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSessionState
}

// ServerSession returns the current server session index (0 = not joined yet, 1 = first server, etc.).
func (m *Manager) ServerSession() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.serverSessionIndex
}

// Record processes a detected Event, handles session state transitions, and detects latent desync chains.
func (m *Manager) Record(event models.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventCounts[event.Type]++
	m.severityCounts[event.Severity]++
	evCopy := event
	evCopy.SessionState = m.currentSessionState
	evCopy.ServerSession = m.serverSessionIndex
	m.lastEvent = &evCopy
	m.recentEventAt = time.Now()

	switch event.Type {
	case models.EventStateTransition:
		switch {
		case event.Details == "State Transition: Main Menu / FrontEnd":
			m.currentSessionState = models.SessionStateMenu

		case event.Details == "State Transition: Joining Server (DGS Handshake)":
			if m.currentSessionState != models.SessionStateJoiningServer {
				m.serverSessionIndex++
				m.currentSessionState = models.SessionStateJoiningServer
				// Reset in-world temporary markers for new server
				m.objectiveSpawned = make(map[string]bool)
			}

		case event.Details == "State Transition: Gameplay (In-World)":
			m.currentSessionState = models.SessionStateGameplay
		}

	case models.EventServerDisconnect:
		m.currentSessionState = models.SessionStateMenu
		// Clear in-world temporary markers upon exiting to menu
		m.objectiveSpawned = make(map[string]bool)

	case models.EventMissionAccepted:
		if m.currentSessionState != models.SessionStateGameplay {
			m.currentSessionState = models.SessionStateGameplay
		}
		if event.MissionID != "" {
			m.activeMissions[event.MissionID] = true
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Accepted"
			m.recordContractAcceptance(event.MissionID, event.Timestamp, event.Details)
		}

	case models.EventObjectiveSpawned:
		if m.currentSessionState != models.SessionStateGameplay {
			m.currentSessionState = models.SessionStateGameplay
		}
		if event.MissionID != "" {
			m.objectiveSpawned[event.MissionID] = true
		}

	case models.EventMissionCompleted:
		if event.MissionID != "" {
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Completed"
			m.recordContractCompletion(event.MissionID, "Completed", event.Timestamp, event.Details)
		}

	case models.EventMissionFailed:
		if event.MissionID != "" {
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Failed"
			m.recordContractCompletion(event.MissionID, "Failed", event.Timestamp, event.Details)
		}

	case models.EventMissionAbandoned:
		if event.MissionID != "" {
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Abandoned"
			m.recordContractCompletion(event.MissionID, "Abandoned", event.Timestamp, event.Details)
		}

	case models.EventContractStatus:
		mid := event.MissionID
		if mid == "" {
			mid = m.lastMissionID
		}
		if mid != "" {
			m.recordContractGeneratorStatus(mid, event.Details)
		}

	case models.EventMissionResult:
		mid := event.MissionID
		if mid == "" {
			mid = m.lastMissionID
		}
		if mid != "" {
			m.recordContractResultOutcome(mid, event.Details)
		}

	case models.EventDGSFailure:
		m.dgsFailureCount++
		// If DGS fails while missions are active, flag potential desync
		for mid := range m.activeMissions {
			m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
				MissionID:      mid,
				IncidentAt:     event.Timestamp,
				Description:    "DGS RPC registration failed during active mission lifecycle",
				TriggerLine:    event.RawLine,
				IsStartupError: false,
				SessionState:   m.currentSessionState,
				ServerSession:  m.serverSessionIndex,
			})
		}

	case models.EventSubsumptionError, models.EventSubsumptionMismatch:
		m.subsumptionErrCount++

		// 1. KEY SYMPTOM FLAG: Check for Application Launch Subsumption failure
		if m.currentSessionState == models.SessionStateAppLaunch || m.currentSessionState == models.SessionStateMenu {
			m.startupSubsumptionErrCount++
			m.isStartupSubsumptionBroken = true

			desc := fmt.Sprintf("CRITICAL SYMPTOM: Client Subsumption failed during %s (GUID: %s). Local AI subsystem is degraded for this session.", m.currentSessionState, event.ErrorGUID)
			histLink := ""

			// Correlate with historical contract from previous sessions
			if hist, exists := m.historicalContracts[event.ErrorGUID]; exists {
				extraInfo := ""
				if hist.GeneratorStatus != "" {
					extraInfo += fmt.Sprintf(", GenStatus: %s", hist.GeneratorStatus)
				}
				if hist.ResultOutcome != "" {
					extraInfo += fmt.Sprintf(", Result: %s", hist.ResultOutcome)
				}
				desc = fmt.Sprintf("SMOKING GUN (CAUSAL LINK PROVEN): Subsumption failure on launch for GUID %s matches Contract [%s%s] from prior session!", event.ErrorGUID, hist.LastStatus, extraInfo)
				histLink = fmt.Sprintf("Linked to contract %s [Status: %s, Completed: %s]", hist.MissionID, hist.LastStatus, hist.CompletedAt)
			}

			m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
				MissionID:      event.ErrorGUID,
				IncidentAt:     event.Timestamp,
				Description:    desc,
				TriggerLine:    event.RawLine,
				IsStartupError: true,
				HistoricalLink: histLink,
				SessionState:   m.currentSessionState,
				ServerSession:  m.serverSessionIndex,
			})
		} else {
			// 2. Gameplay Desync Chain: Active mission encountering Subsumption failure
			for mid := range m.activeMissions {
				hasObjective := m.objectiveSpawned[mid]
				desc := "Subsumption service failure detected while mission is active in gameplay"
				if !hasObjective {
					desc = "Smoking Gun: Subsumption failure occurred before mission objective marker manifested (Ephemeral Player bug)"
				}
				m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
					MissionID:      mid,
					IncidentAt:     event.Timestamp,
					Description:    desc,
					TriggerLine:    event.RawLine,
					IsStartupError: false,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			}
		}
	}
}

func (m *Manager) recordContractAcceptance(missionID, timestamp, details string) {
	if missionID == "" {
		return
	}
	c := m.historicalContracts[missionID]
	c.MissionID = missionID
	c.LastStatus = "Accepted"
	c.AcceptedAt = timestamp
	c.RecordedAt = time.Now().UTC()
	c.Details = details
	c.LaunchIndex = m.serverSessionIndex
	m.historicalContracts[missionID] = c
	m.flushPersistence()
}

func (m *Manager) recordContractCompletion(missionID, status, timestamp, details string) {
	if missionID == "" {
		return
	}
	c := m.historicalContracts[missionID]
	c.MissionID = missionID
	c.LastStatus = status
	c.CompletedAt = timestamp
	c.RecordedAt = time.Now().UTC()
	c.Details = details
	c.LaunchIndex = m.serverSessionIndex
	m.historicalContracts[missionID] = c
	m.flushPersistence()
}

func (m *Manager) recordContractGeneratorStatus(missionID, details string) {
	if missionID == "" {
		return
	}
	c := m.historicalContracts[missionID]
	c.MissionID = missionID
	c.GeneratorStatus = details
	m.historicalContracts[missionID] = c
	m.flushPersistence()
}

func (m *Manager) recordContractResultOutcome(missionID, details string) {
	if missionID == "" {
		return
	}
	c := m.historicalContracts[missionID]
	c.MissionID = missionID
	c.ResultOutcome = details
	m.historicalContracts[missionID] = c
	m.flushPersistence()
}

func (m *Manager) flushPersistence() {
	if m.persistentFilePath != "" {
		_ = m.savePersistentState(m.persistentFilePath)
	}
}

func (m *Manager) loadPersistentState(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var list []models.HistoricalContract
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	for _, item := range list {
		m.historicalContracts[item.MissionID] = item
	}
	return nil
}

func (m *Manager) savePersistentState(path string) error {
	list := make([]models.HistoricalContract, 0, len(m.historicalContracts))
	for _, v := range m.historicalContracts {
		list = append(list, v)
	}

	bytes, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bytes, 0644)
}

// Snapshot returns an atomic, read-only copy of current metrics for UI and forensic output.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	evCounts := make(map[models.EventType]int, len(m.eventCounts))
	for k, v := range m.eventCounts {
		evCounts[k] = v
	}

	sevCounts := make(map[models.Severity]int, len(m.severityCounts))
	for k, v := range m.severityCounts {
		sevCounts[k] = v
	}

	var incidents []DesyncIncident
	if len(m.desyncIncidents) > 0 {
		incidents = make([]DesyncIncident, len(m.desyncIncidents))
		copy(incidents, m.desyncIncidents)
	}

	var lastEv *models.Event
	if m.lastEvent != nil {
		ev := *m.lastEvent
		lastEv = &ev
	}

	return Snapshot{
		TotalLinesProcessed:        m.totalLinesProcessed,
		EventCounts:                evCounts,
		SeverityCounts:             sevCounts,
		LastEvent:                  lastEv,
		LastMissionID:              m.lastMissionID,
		LastMissionStatus:          m.lastMissionStatus,
		DGSFailureCount:            m.dgsFailureCount,
		SubsumptionErrCount:        m.subsumptionErrCount,
		StartupSubsumptionErrCount: m.startupSubsumptionErrCount,
		IsStartupSubsumptionBroken: m.isStartupSubsumptionBroken,
		CurrentSessionState:        m.currentSessionState,
		ServerSessionIndex:         m.serverSessionIndex,
		DesyncIncidents:            incidents,
		RecentEventAt:              m.recentEventAt,
	}
}
