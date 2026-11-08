package models

import "time"

// EventType defines the categorized classification of a detected log event.
type EventType string

const (
	// EventMissionAccepted indicates a contract was accepted by the player.
	EventMissionAccepted EventType = "Mission Accepted"
	// EventMissionCompleted indicates a contract reached a successful completion state.
	EventMissionCompleted EventType = "Mission Completed"
	// EventMissionFailed indicates a contract failed cleanly in game logic.
	EventMissionFailed EventType = "Mission Failed"
	// EventMissionAbandoned indicates a user-initiated mission cancellation.
	EventMissionAbandoned EventType = "Mission Abandoned"
	// EventObjectiveSpawned indicates physical marker manifestation in the game world.
	EventObjectiveSpawned EventType = "Objective Spawned"
	// EventContractInit indicates the contract generator subsystem has initialized.
	EventContractInit EventType = "Contract Init"
	// EventContractStatus indicates a state transition logged by CContractGenerator.
	EventContractStatus EventType = "Contract Status"
	// EventMissionResult indicates the final reported outcome of a contract or mission.
	EventMissionResult EventType = "Mission Result"
	// EventDGSFailure indicates a failure to register RPC in Distributed Game Services.
	EventDGSFailure EventType = "DGS Failure"
	// EventSubsumptionError indicates an AI Subsumption ErrorReporter diagnostic.
	EventSubsumptionError EventType = "Subsumption Error"
	// EventSubsumptionMismatch indicates missing Subsumption tasks, variables, or archetypes.
	EventSubsumptionMismatch EventType = "Subsumption Mismatch"
	// EventStateTransition indicates a lifecycle phase change (App Launch, Menu, Joining Server, Gameplay).
	EventStateTransition EventType = "State Transition"
	// EventServerDisconnect indicates the client disconnected from a server or exited to menu.
	EventServerDisconnect EventType = "Server Disconnect"
)

// SessionState represents the macro operational phase of the Star Citizen client.
type SessionState string

const (
	// SessionStateAppLaunch indicates initial engine startup and client initialization.
	SessionStateAppLaunch SessionState = "Application Launch"
	// SessionStateMenu indicates the client has returned to the main menu or front end.
	SessionStateMenu SessionState = "Main Menu"
	// SessionStateJoiningServer indicates connection handshakes, entity loading, and subsystem init.
	SessionStateJoiningServer SessionState = "Joining Server"
	// SessionStateGameplay indicates active player presence in the game world.
	SessionStateGameplay SessionState = "Gameplay"
)

// Severity represents the operational impact level of an event.
type Severity string

const (
	// SeverityInfo designates informational lifecycle transitions.
	SeverityInfo Severity = "INFO"
	// SeverityNotice designates system initialization or state notices.
	SeverityNotice Severity = "NOTICE"
	// SeverityWarning designates recoverable failures or user cancellations.
	SeverityWarning Severity = "WARNING"
	// SeverityError designates logic errors such as Subsumption UUID failures.
	SeverityError Severity = "ERROR"
	// SeverityCritical designates service drops or crash-inducing RPC failures.
	SeverityCritical Severity = "CRITICAL"
)

// Event holds structured forensic data extracted from a raw Star Citizen log line.
type Event struct {
	Timestamp     string
	Type          EventType
	Severity      Severity
	MissionID     string
	ErrorGUID     string
	Details       string
	RawLine       string
	ParsedAt      time.Time
	SessionState  SessionState
	ServerSession int
}

// RawLogLine represents a single line of log text read from the source stream.
type RawLogLine struct {
	LineNumber int
	Content    string
}

// HistoricalContract persists completed or active mission state across application launches.
type HistoricalContract struct {
	MissionID       string    `json:"mission_id"`
	LastStatus      string    `json:"last_status"`
	AcceptedAt      string    `json:"accepted_at,omitempty"`
	CompletedAt     string    `json:"completed_at,omitempty"`
	ResultOutcome   string    `json:"result_outcome,omitempty"`
	GeneratorStatus string    `json:"generator_status,omitempty"`
	RecordedAt      time.Time `json:"recorded_at"`
	Details         string    `json:"details"`
	LaunchIndex     int       `json:"launch_index"`
}
