package state

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

const (
	maxRecentEvents = 15
	maxStateChanges = 12
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

// ContractEntry represents a single accepted contract and its final outcome within a session.
type ContractEntry struct {
	LogBanner    string
	ServerSess   int
	MissionID    string
	ContractName string
	Blueprint    string
	AcceptedAt   string
	EndedAt      string
	Result       string
	AcceptCount  int
	Anomalies    []string
}

// Snapshot provides a point-in-time immutable view of the log watcher telemetry.
type Snapshot struct {
	TotalLinesProcessed        int
	EventCounts                map[models.EventType]int
	SeverityCounts             map[models.Severity]int
	LastEvent                  *models.Event
	RecentEvents               []models.Event
	StateChanges               []models.Event
	LastMissionID              string
	LastMissionStatus          string
	DGSFailureCount            int
	SubsumptionErrCount        int
	StartupSubsumptionErrCount int
	IsStartupSubsumptionBroken bool
	CurrentSessionState        models.SessionState
	ServerSessionIndex         int
	DesyncIncidents            []DesyncIncident
	ContractLedger             []ContractEntry
	RecentEventAt              time.Time
}

// Manager maintains thread-safe live telemetry, macro session states, and causal state for Star Citizen.
type Manager struct {
	mu                         sync.RWMutex
	totalLinesProcessed        int
	eventCounts                map[models.EventType]int
	severityCounts             map[models.Severity]int
	lastEvent                  *models.Event
	recentEvents               []models.Event
	stateChanges               []models.Event
	lastMissionID              string
	lastMissionStatus          string
	activeMissions             map[string]bool
	objectiveSpawned           map[string]bool
	contractLedger             []ContractEntry
	activeContractIdx          map[string]int
	completedContractIdx       map[string]int
	contractBlueprints         map[string]string
	markerDataBankAdds         map[string]int
	markerDataBankRemoves      map[string]int
	locationStormCount         int
	locationStormIncidentIdx   int
	lastLocationStormPrefix    string
	currentLogBanner           string
	hyperionMissionID          string
	hyperionAcceptedAt         string
	hyperionAcceptLine         string
	hyperionReachedEngineering bool
	hyperionEngineeringLine    string
	hyperionDeferredExplosions int
	hyperionInteriorTriggered  bool
	hyperionInvestigateEnabled bool
	hyperionIncidentRecorded   bool
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

// NewManager creates an in-memory thread-safe StateManager instance with default AppLaunch state.
func NewManager() *Manager {
	return NewManagerWithPersistence("")
}

// NewManagerWithPersistence creates an initialized StateManager with custom persistence path.
func NewManagerWithPersistence(persistPath string) *Manager {
	m := &Manager{
		eventCounts:              make(map[models.EventType]int),
		severityCounts:           make(map[models.Severity]int),
		activeMissions:           make(map[string]bool),
		objectiveSpawned:         make(map[string]bool),
		activeContractIdx:        make(map[string]int),
		completedContractIdx:     make(map[string]int),
		contractBlueprints:       make(map[string]string),
		markerDataBankAdds:       make(map[string]int),
		markerDataBankRemoves:    make(map[string]int),
		locationStormIncidentIdx: -1,
		currentSessionState:      models.SessionStateAppLaunch,
		serverSessionIndex:       0,
		historicalContracts:      make(map[string]models.HistoricalContract),
		persistentFilePath:       persistPath,
		desyncIncidents:          nil,
		recentEvents:             nil,
		stateChanges:             nil,
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

// Record processes a detected Event, handles session state transitions, and tracks recent plain-English events.
func (m *Manager) Record(event models.Event) {
	_, _ = m.RecordAndEnrich(event)
}

// RecordAndEnrich processes a detected Event, enriches it with session state and plain-English meaning,
// and returns false if the event is a redundant consecutive state transition.
func (m *Manager) RecordAndEnrich(event models.Event) (models.Event, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if banner := extractBackupBanner(event.RawLine); banner != "" {
		m.finalizeHyperionCheck(event.Timestamp)
		m.currentLogBanner = banner
	}

	targetState := resolveTargetState(event)
	if (event.Type == models.EventStateTransition || event.Type == models.EventServerDisconnect) &&
		targetState != "" && m.currentSessionState == targetState {
		// Suppress duplicate consecutive transitions into the exact same session state
		return event, false
	}

	prevState := m.currentSessionState

	switch event.Type {
	case models.EventStateTransition, models.EventServerDisconnect:
		if targetState != "" {
			if targetState != models.SessionStateGameplay {
				m.finalizeHyperionCheck(event.Timestamp)
			}
			if targetState == models.SessionStateJoiningServer && m.currentSessionState != models.SessionStateJoiningServer {
				m.serverSessionIndex++
				m.resetSessionTrackers()
			}
			if targetState == models.SessionStateMenu || targetState == models.SessionStateAppLaunch {
				m.resetSessionTrackers()
			}
			m.currentSessionState = targetState
		}

	case models.EventMissionAccepted:
		if m.currentSessionState != models.SessionStateGameplay {
			m.currentSessionState = models.SessionStateGameplay
		}
		title := extractContractTitle(event.RawLine)
		if title == "" {
			title = extractContractTitle(event.Details)
		}
		if title != "" && event.MissionID != "" {
			event.Meaning = fmt.Sprintf("Player accepted a contract: %q [Mission: %s]", title, event.MissionID)
		} else if title != "" {
			event.Meaning = fmt.Sprintf("Player accepted a contract: %q", title)
		}
		if event.MissionID != "" {
			m.activeMissions[event.MissionID] = true
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Accepted"
			m.recordContractAcceptance(event.MissionID, event.Timestamp, event.Details)
			m.trackLedgerAccept(event.MissionID, title, event.Timestamp)
		}
		if strings.Contains(strings.ToLower(event.RawLine), "project hyperion") || strings.Contains(strings.ToLower(event.Details), "project hyperion") {
			m.finalizeHyperionCheck(event.Timestamp)
			m.hyperionMissionID = event.MissionID
			if m.hyperionMissionID == "" {
				m.hyperionMissionID = "Project Hyperion"
			}
			m.hyperionAcceptedAt = event.Timestamp
			m.hyperionAcceptLine = event.RawLine
			m.hyperionReachedEngineering = false
			m.hyperionEngineeringLine = ""
			m.hyperionDeferredExplosions = 0
			m.hyperionInteriorTriggered = false
			m.hyperionInvestigateEnabled = true
			m.hyperionIncidentRecorded = false
		}

	case models.EventObjectiveSpawned:
		if m.currentSessionState != models.SessionStateGameplay {
			m.currentSessionState = models.SessionStateGameplay
		}
		if event.MissionID != "" {
			m.objectiveSpawned[event.MissionID] = true
			bp := extractContractBlueprint(event.RawLine)
			if bp == "" {
				bp = extractContractBlueprint(event.Details)
			}
			if bp != "" {
				m.contractBlueprints[event.MissionID] = bp
				if idx, exists := m.activeContractIdx[event.MissionID]; exists && idx < len(m.contractLedger) {
					if m.contractLedger[idx].Blueprint == "" {
						m.contractLedger[idx].Blueprint = bp
					}
				}
			}
		}

	case models.EventMissionCompleted:
		if event.MissionID != "" {
			if event.MissionID == m.hyperionMissionID {
				m.clearHyperionTracker()
			}
			title := extractContractTitle(event.RawLine)
			if title == "" {
				title = extractContractTitle(event.Details)
			}
			if title != "" {
				event.Meaning = fmt.Sprintf("Contract completed: %q [Mission: %s]", title, event.MissionID)
			}
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Completed"
			m.recordContractCompletion(event.MissionID, "Completed", event.Timestamp, event.Details)
			if m.trackLedgerOutcome(event.MissionID, "Completed", title, event.Timestamp) {
				m.eventCounts[event.Type]++
				m.severityCounts[event.Severity]++
				return event, false
			}
		}

	case models.EventMissionFailed:
		if event.MissionID != "" {
			if event.MissionID == m.hyperionMissionID {
				m.finalizeHyperionCheck(event.Timestamp)
			}
			title := extractContractTitle(event.RawLine)
			if title == "" {
				title = extractContractTitle(event.Details)
			}
			if title != "" {
				event.Meaning = fmt.Sprintf("Contract failed: %q [Mission: %s]", title, event.MissionID)
			}
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Failed"
			m.recordContractCompletion(event.MissionID, "Failed", event.Timestamp, event.Details)
			if m.trackLedgerOutcome(event.MissionID, "Failed", title, event.Timestamp) {
				m.eventCounts[event.Type]++
				m.severityCounts[event.Severity]++
				return event, false
			}
		}

	case models.EventMissionAbandoned:
		if event.MissionID != "" {
			if event.MissionID == m.hyperionMissionID {
				m.finalizeHyperionCheck(event.Timestamp)
			}
			title := extractContractTitle(event.RawLine)
			if title == "" {
				title = extractContractTitle(event.Details)
			}
			if title != "" {
				event.Meaning = fmt.Sprintf("Contract abandoned: %q [Mission: %s]", title, event.MissionID)
			}
			delete(m.activeMissions, event.MissionID)
			m.lastMissionID = event.MissionID
			m.lastMissionStatus = "Abandoned"
			m.recordContractCompletion(event.MissionID, "Abandoned", event.Timestamp, event.Details)
			if m.trackLedgerOutcome(event.MissionID, "Abandoned", title, event.Timestamp) {
				m.eventCounts[event.Type]++
				m.severityCounts[event.Severity]++
				return event, false
			}
		}

	case models.EventHyperionStage:
		if strings.Contains(event.RawLine, "Retrieve One of the Reagent Codes in Engineering Wing") {
			if m.hyperionMissionID == "" {
				m.hyperionMissionID = event.MissionID
				if m.hyperionMissionID == "" {
					m.hyperionMissionID = "Project Hyperion"
				}
				m.hyperionAcceptedAt = event.Timestamp
				m.hyperionAcceptLine = event.RawLine
			}
			m.hyperionReachedEngineering = true
			m.hyperionEngineeringLine = event.RawLine
			m.hyperionInvestigateEnabled = event.ActiveInvestigation
			event.ActiveInvestigation = false
		} else if strings.Contains(event.RawLine, "Obtain Thermal") {
			m.hyperionInteriorTriggered = true
			event.ActiveInvestigation = false
		}

	case models.EventDeferredExplosion:
		m.hyperionDeferredExplosions++
		event.ActiveInvestigation = false
		if m.hyperionDeferredExplosions > 1 {
			m.eventCounts[event.Type]++
			m.severityCounts[event.Severity]++
			return event, false
		}

	case models.EventMissingDataBank:
		if m.currentSessionState != models.SessionStateGameplay {
			// Ignore benign marker cleanup after local player entity is destroyed during Main Menu disconnect
			event.ActiveInvestigation = false
			event.Severity = models.SeverityNotice
			event.Meaning = "Objective marker cleaned up after player disconnect (Main Menu teardown)"
		} else {
			m.appendContractAnomaly(m.lastMissionID, "Missing ObjectDataBank")
			if event.ActiveInvestigation {
				targetID := m.lastMissionID
				if targetID == "" {
					targetID = "Account/Session"
				}
				m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
					MissionID:      targetID,
					IncidentAt:     event.Timestamp,
					Description:    m.withLogBanner(event.Meaning),
					TriggerLine:    event.RawLine,
					IsStartupError: false,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			}
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

	case models.EventMarkerDataBankAdded:
		markerID := extractMarkerID(event.RawLine, event.Meaning)
		m.markerDataBankAdds[markerID]++
		event.ActiveInvestigation = false
		if m.markerDataBankAdds[markerID] > 1 {
			m.eventCounts[event.Type]++
			m.severityCounts[event.Severity]++
			return event, false
		}

	case models.EventMarkerDataBankRemoved:
		markerID := extractMarkerID(event.RawLine, event.Meaning)
		m.markerDataBankRemoves[markerID]++
		adds := m.markerDataBankAdds[markerID]
		removes := m.markerDataBankRemoves[markerID]
		if adds == 0 && event.ActiveInvestigation {
			m.appendContractAnomaly(m.lastMissionID, "Orphan Marker Unbind")
			if removes == 1 {
				targetMid := m.lastMissionID
				if targetMid == "" {
					targetMid = markerID
				}
				desc := fmt.Sprintf("ACTIVE INVESTIGATION: Objective marker %s immediately removed from PlayerDataBank on stream-in without ever being added!", markerID)
				event.Meaning = desc
				m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
					MissionID:      targetMid,
					IncidentAt:     event.Timestamp,
					Description:    m.withLogBanner(desc),
					TriggerLine:    event.RawLine,
					IsStartupError: false,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			} else {
				m.eventCounts[event.Type]++
				m.severityCounts[event.Severity]++
				return event, false
			}
		} else {
			event.ActiveInvestigation = false
			event.Meaning = fmt.Sprintf("Objective marker unbound from player DataBank (%s)", markerID)
			if removes > 1 {
				m.eventCounts[event.Type]++
				m.severityCounts[event.Severity]++
				return event, false
			}
		}

	case models.EventLocationStorm:
		m.locationStormCount++
		if event.ActiveInvestigation && m.locationStormCount >= 20 {
			targetMid := m.lastMissionID
			if targetMid == "" {
				targetMid = "TheCollector_*"
			}
			desc := m.withLogBanner(fmt.Sprintf("ACTIVE INVESTIGATION: Contract generator storm recalculating Wikelo/Collector locations (%dx in session)", m.locationStormCount))
			if m.locationStormIncidentIdx >= 0 && m.locationStormIncidentIdx < len(m.desyncIncidents) {
				m.desyncIncidents[m.locationStormIncidentIdx].Description = desc
			} else {
				m.locationStormIncidentIdx = len(m.desyncIncidents)
				m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
					MissionID:      targetMid,
					IncidentAt:     event.Timestamp,
					Description:    desc,
					TriggerLine:    event.RawLine,
					IsStartupError: false,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			}
		} else {
			event.ActiveInvestigation = false
		}
		if m.locationStormCount < 20 {
			m.eventCounts[event.Type]++
			m.severityCounts[event.Severity]++
			return event, false
		}
		prefix := event.Timestamp
		if len(prefix) > 16 {
			prefix = prefix[:16]
		}
		if prefix != "" && prefix == m.lastLocationStormPrefix {
			m.eventCounts[event.Type]++
			m.severityCounts[event.Severity]++
			return event, false
		}
		m.lastLocationStormPrefix = prefix

	case models.EventDGSFailure:
		m.dgsFailureCount++
		if event.ActiveInvestigation {
			for mid := range m.activeMissions {
				m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
					MissionID:      mid,
					IncidentAt:     event.Timestamp,
					Description:    m.withLogBanner("DGS RPC registration failed during active mission lifecycle"),
					TriggerLine:    event.RawLine,
					IsStartupError: false,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			}
		}

	case models.EventSubsumptionError, models.EventSubsumptionMismatch:
		m.subsumptionErrCount++
		if event.ActiveInvestigation {
			if m.currentSessionState == models.SessionStateAppLaunch || m.currentSessionState == models.SessionStateMenu {
				m.startupSubsumptionErrCount++
				m.isStartupSubsumptionBroken = true

				desc := fmt.Sprintf("CRITICAL SYMPTOM: Client Subsumption failed during %s (GUID: %s). Local AI subsystem is degraded for this session.", m.currentSessionState, event.ErrorGUID)
				histLink := ""

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
					Description:    m.withLogBanner(desc),
					TriggerLine:    event.RawLine,
					IsStartupError: true,
					HistoricalLink: histLink,
					SessionState:   m.currentSessionState,
					ServerSession:  m.serverSessionIndex,
				})
			} else {
				for mid := range m.activeMissions {
					hasObjective := m.objectiveSpawned[mid]
					desc := "Subsumption service failure detected while mission is active in gameplay"
					if !hasObjective {
						desc = "Smoking Gun: Subsumption failure occurred before mission objective marker manifested (Ephemeral Player bug)"
					}
					m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
						MissionID:      mid,
						IncidentAt:     event.Timestamp,
						Description:    m.withLogBanner(desc),
						TriggerLine:    event.RawLine,
						IsStartupError: false,
						SessionState:   m.currentSessionState,
						ServerSession:  m.serverSessionIndex,
					})
				}
			}
		}

	default:
		if event.Type == models.EventProximityLoss {
			m.appendContractAnomaly(event.MissionID, "Proximity Loss ('Return to' Loop)")
		}
		if event.ActiveInvestigation {
			targetID := event.MissionID
			if targetID == "" {
				targetID = event.ErrorGUID
			}
			if targetID == "" {
				targetID = m.lastMissionID
			}
			if targetID == "" {
				targetID = "Account/Session"
			}
			desc := event.Meaning
			if desc == "" {
				desc = string(event.Type)
			}
			m.desyncIncidents = append(m.desyncIncidents, DesyncIncident{
				MissionID:      targetID,
				IncidentAt:     event.Timestamp,
				Description:    m.withLogBanner(desc),
				TriggerLine:    event.RawLine,
				IsStartupError: m.currentSessionState == models.SessionStateAppLaunch || m.currentSessionState == models.SessionStateMenu,
				SessionState:   m.currentSessionState,
				ServerSession:  m.serverSessionIndex,
			})
		}
	}

	m.eventCounts[event.Type]++
	m.severityCounts[event.Severity]++

	evCopy := event
	evCopy.SessionState = m.currentSessionState
	evCopy.ServerSession = m.serverSessionIndex
	if evCopy.Meaning == "" {
		if evCopy.Details != "" {
			evCopy.Meaning = evCopy.Details
		} else {
			evCopy.Meaning = string(evCopy.Type)
		}
	}
	if prevState != m.currentSessionState && (event.Type == models.EventStateTransition || event.Type == models.EventServerDisconnect) {
		evCopy.Meaning = fmt.Sprintf("%s -> %s (%s)", prevState, m.currentSessionState, evCopy.Meaning)
	}

	m.lastEvent = &evCopy
	m.recentEventAt = time.Now()

	m.recentEvents = appendBounded(m.recentEvents, evCopy, maxRecentEvents)
	if isStateOrContractChange(evCopy.Type) || evCopy.ActiveInvestigation {
		m.stateChanges = appendBounded(m.stateChanges, evCopy, maxStateChanges)
	}

	return evCopy, true
}

func (m *Manager) resetSessionTrackers() {
	m.finalizeActiveContracts("Disconnected")
	m.objectiveSpawned = make(map[string]bool)
	m.markerDataBankAdds = make(map[string]int)
	m.markerDataBankRemoves = make(map[string]int)
	m.locationStormCount = 0
	m.locationStormIncidentIdx = -1
	m.lastLocationStormPrefix = ""
	m.clearHyperionTracker()
}

func (m *Manager) clearHyperionTracker() {
	m.hyperionMissionID = ""
	m.hyperionAcceptedAt = ""
	m.hyperionAcceptLine = ""
	m.hyperionReachedEngineering = false
	m.hyperionEngineeringLine = ""
	m.hyperionDeferredExplosions = 0
	m.hyperionInteriorTriggered = false
	m.hyperionInvestigateEnabled = false
	m.hyperionIncidentRecorded = false
}

func (m *Manager) finalizeHyperionCheck(timestamp string) {
	if inc, ok := m.buildHyperionIncident(timestamp); ok {
		m.desyncIncidents = append(m.desyncIncidents, inc)
		m.hyperionIncidentRecorded = true
		m.appendContractAnomaly(m.hyperionMissionID, "HYPERION STALL (No Worm Spawn)")
		if idx, exists := m.activeContractIdx[m.hyperionMissionID]; exists && idx < len(m.contractLedger) {
			if m.contractLedger[idx].Result == "In Progress" {
				m.contractLedger[idx].Result = "Stalled (Hyperion Bug)"
			}
		}
	}
	m.clearHyperionTracker()
}

func (m *Manager) buildHyperionIncident(timestamp string) (DesyncIncident, bool) {
	if m.hyperionMissionID == "" || m.hyperionIncidentRecorded || !m.hyperionInvestigateEnabled {
		return DesyncIncident{}, false
	}
	if m.hyperionDeferredExplosions > 0 || m.hyperionInteriorTriggered {
		return DesyncIncident{}, false
	}

	ts := timestamp
	if ts == "" {
		ts = m.hyperionAcceptedAt
	}
	triggerLine := m.hyperionEngineeringLine
	if triggerLine == "" {
		triggerLine = m.hyperionAcceptLine
	}

	var desc string
	if m.hyperionReachedEngineering {
		desc = "ACTIVE INVESTIGATION (HYPERION STALL): Accepted Project Hyperion and reached Engineering Wing objective, but 0 CDeferredExplosionEffect (worm spawn) or interior sub-objective triggers fired before session/mission end!"
	} else {
		desc = "ACTIVE INVESTIGATION (HYPERION STALL): Accepted Project Hyperion, but 0 CDeferredExplosionEffect (worm spawn) or interior sub-objective triggers fired before session/mission end!"
	}

	return DesyncIncident{
		MissionID:      m.hyperionMissionID,
		IncidentAt:     ts,
		Description:    m.withLogBanner(desc),
		TriggerLine:    triggerLine,
		IsStartupError: false,
		SessionState:   models.SessionStateGameplay,
		ServerSession:  m.serverSessionIndex,
	}, true
}

func extractBackupBanner(rawLine string) string {
	const key = `BackupNameAttachment="`
	idx := strings.Index(rawLine, key)
	if idx < 0 {
		return ""
	}
	sub := rawLine[idx+len(key):]
	end := strings.Index(sub, `"`)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(sub[:end])
}

func (m *Manager) withLogBanner(desc string) string {
	if m.currentLogBanner == "" {
		return desc
	}
	return fmt.Sprintf("[Log: %s] %s", m.currentLogBanner, desc)
}

func (m *Manager) trackLedgerAccept(missionID, title, timestamp string) {
	if missionID == "" {
		return
	}
	if idx, exists := m.activeContractIdx[missionID]; exists && idx < len(m.contractLedger) {
		m.contractLedger[idx].AcceptCount++
		if title != "" && (m.contractLedger[idx].ContractName == "" || m.contractLedger[idx].ContractName == "Unknown Contract") {
			m.contractLedger[idx].ContractName = title
		}
		rebroadcastNote := fmt.Sprintf("Re-broadcast Contract Accepted (%dx)", m.contractLedger[idx].AcceptCount)
		replaced := false
		for i, a := range m.contractLedger[idx].Anomalies {
			if strings.HasPrefix(a, "Re-broadcast Contract Accepted") {
				m.contractLedger[idx].Anomalies[i] = rebroadcastNote
				replaced = true
				break
			}
		}
		if !replaced {
			m.contractLedger[idx].Anomalies = append(m.contractLedger[idx].Anomalies, rebroadcastNote)
		}
		return
	}

	if title == "" {
		title = "Unknown Contract"
	}
	entry := ContractEntry{
		LogBanner:    m.currentLogBanner,
		ServerSess:   m.serverSessionIndex,
		MissionID:    missionID,
		ContractName: title,
		Blueprint:    m.contractBlueprints[missionID],
		AcceptedAt:   timestamp,
		Result:       "In Progress",
		AcceptCount:  1,
	}
	m.contractLedger = append(m.contractLedger, entry)
	m.activeContractIdx[missionID] = len(m.contractLedger) - 1
	delete(m.completedContractIdx, missionID)
}

func (m *Manager) trackLedgerOutcome(missionID, status, title, timestamp string) bool {
	if missionID == "" {
		return false
	}
	if idx, exists := m.activeContractIdx[missionID]; exists && idx < len(m.contractLedger) {
		m.contractLedger[idx].Result = status
		m.contractLedger[idx].EndedAt = timestamp
		if title != "" && (m.contractLedger[idx].ContractName == "" || m.contractLedger[idx].ContractName == "Unknown Contract") {
			m.contractLedger[idx].ContractName = title
		}
		delete(m.activeContractIdx, missionID)
		m.completedContractIdx[missionID] = idx
		return false
	}
	if idx, exists := m.completedContractIdx[missionID]; exists && idx < len(m.contractLedger) {
		if title != "" && (m.contractLedger[idx].ContractName == "" || m.contractLedger[idx].ContractName == "Unknown Contract") {
			m.contractLedger[idx].ContractName = title
		}
		return true
	}
	return false
}

func (m *Manager) appendContractAnomaly(missionID, anomaly string) {
	if anomaly == "" {
		return
	}
	targetID := missionID
	if targetID == "" {
		targetID = m.lastMissionID
	}
	if targetID == "" {
		return
	}
	idx, exists := m.activeContractIdx[targetID]
	if !exists || idx >= len(m.contractLedger) {
		return
	}
	for _, a := range m.contractLedger[idx].Anomalies {
		if a == anomaly {
			return
		}
	}
	m.contractLedger[idx].Anomalies = append(m.contractLedger[idx].Anomalies, anomaly)
}

func (m *Manager) finalizeActiveContracts(reason string) {
	for mid, idx := range m.activeContractIdx {
		if idx < len(m.contractLedger) && m.contractLedger[idx].Result == "In Progress" {
			if len(m.contractLedger[idx].Anomalies) > 0 {
				m.contractLedger[idx].Result = fmt.Sprintf("Stalled / Unfinished (%s)", reason)
			} else {
				m.contractLedger[idx].Result = fmt.Sprintf("Unfinished (%s)", reason)
			}
		}
		delete(m.activeContractIdx, mid)
	}
	m.completedContractIdx = make(map[string]int)
}

func extractContractTitle(rawLine string) string {
	if rawLine == "" {
		return ""
	}
	prefixes := []string{
		"Contract Accepted:",
		"Contract Complete:",
		"Contract Failed:",
		"Contract Abandoned:",
	}
	sub := ""
	for _, p := range prefixes {
		if idx := strings.Index(rawLine, p); idx >= 0 {
			sub = rawLine[idx+len(p):]
			break
		}
	}
	if sub == "" {
		return ""
	}

	if qIdx := strings.Index(sub, `"`); qIdx >= 0 {
		sub = sub[:qIdx]
	} else if mIdx := strings.Index(sub, "MissionId:"); mIdx >= 0 {
		sub = sub[:mIdx]
	}

	for {
		openIdx := strings.Index(sub, "<EM")
		if openIdx < 0 {
			openIdx = strings.Index(sub, "</EM")
		}
		if openIdx < 0 {
			break
		}
		closeIdx := strings.Index(sub[openIdx:], ">")
		if closeIdx < 0 {
			break
		}
		sub = sub[:openIdx] + sub[openIdx+closeIdx+1:]
	}

	sub = strings.TrimSpace(sub)
	sub = strings.TrimSuffix(sub, ":")
	return strings.TrimSpace(sub)
}

func extractContractBlueprint(rawLine string) string {
	const key = "contract ["
	idx := strings.Index(rawLine, key)
	if idx < 0 {
		return ""
	}
	sub := rawLine[idx+len(key):]
	end := strings.Index(sub, "]")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(sub[:end])
}

func extractMarkerID(rawLine string, meaning string) string {
	idx := strings.Index(rawLine, "MissionObjectiveMarker_")
	if idx >= 0 {
		sub := rawLine[idx:]
		end := strings.Index(sub, "]")
		if end >= 0 {
			return sub[:end+1]
		}
	}
	return meaning
}

func resolveTargetState(event models.Event) models.SessionState {
	if event.TargetState != "" {
		return event.TargetState
	}
	switch {
	case event.Type == models.EventServerDisconnect:
		return models.SessionStateMenu
	case event.Details == "State Transition: Main Menu / FrontEnd":
		return models.SessionStateMenu
	case event.Details == "State Transition: Joining Server (DGS Handshake)":
		return models.SessionStateJoiningServer
	case event.Details == "State Transition: Gameplay (In-World)":
		return models.SessionStateGameplay
	default:
		return ""
	}
}

func isStateOrContractChange(t models.EventType) bool {
	switch t {
	case models.EventStateTransition,
		models.EventServerDisconnect,
		models.EventMissionAccepted,
		models.EventObjectiveSpawned,
		models.EventMissionCompleted,
		models.EventMissionFailed,
		models.EventMissionAbandoned,
		models.EventContractStatus,
		models.EventMissionResult:
		return true
	default:
		return false
	}
}

func appendBounded(slice []models.Event, ev models.Event, maxLen int) []models.Event {
	slice = append(slice, ev)
	if len(slice) > maxLen {
		slice = slice[len(slice)-maxLen:]
	}
	return slice
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
	hyperionInc, hasPendingHyperionStall := m.buildHyperionIncident(m.hyperionAcceptedAt)
	if hasPendingHyperionStall {
		incidents = append(incidents, hyperionInc)
	}

	var ledger []ContractEntry
	if len(m.contractLedger) > 0 {
		ledger = make([]ContractEntry, len(m.contractLedger))
		for i, c := range m.contractLedger {
			cCopy := c
			if len(c.Anomalies) > 0 {
				cCopy.Anomalies = make([]string, len(c.Anomalies))
				copy(cCopy.Anomalies, c.Anomalies)
			}
			if hasPendingHyperionStall && cCopy.MissionID == m.hyperionMissionID && cCopy.Result == "In Progress" {
				cCopy.Result = "Stalled (Hyperion Bug)"
				cCopy.Anomalies = append(cCopy.Anomalies, "HYPERION STALL (No Worm Spawn)")
			} else if cCopy.Result == "In Progress" {
				if len(cCopy.Anomalies) > 0 {
					cCopy.Result = "Stalled / Unfinished"
				} else {
					cCopy.Result = "In Progress / Unfinished"
				}
			}
			ledger[i] = cCopy
		}
	}

	var recent []models.Event
	if len(m.recentEvents) > 0 {
		recent = make([]models.Event, len(m.recentEvents))
		copy(recent, m.recentEvents)
	}

	var changes []models.Event
	if len(m.stateChanges) > 0 {
		changes = make([]models.Event, len(m.stateChanges))
		copy(changes, m.stateChanges)
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
		RecentEvents:               recent,
		StateChanges:               changes,
		LastMissionID:              m.lastMissionID,
		LastMissionStatus:          m.lastMissionStatus,
		DGSFailureCount:            m.dgsFailureCount,
		SubsumptionErrCount:        m.subsumptionErrCount,
		StartupSubsumptionErrCount: m.startupSubsumptionErrCount,
		IsStartupSubsumptionBroken: m.isStartupSubsumptionBroken,
		CurrentSessionState:        m.currentSessionState,
		ServerSessionIndex:         m.serverSessionIndex,
		DesyncIncidents:            incidents,
		ContractLedger:             ledger,
		RecentEventAt:              m.recentEventAt,
	}
}
