package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

var defaultTimestampRegex = regexp.MustCompile(`^<(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{3})?Z?)>`)

// Filter defines the interface for evaluating raw log lines against specific bug signatures.
type Filter interface {
	Name() string
	Type() models.EventType
	Severity() models.Severity
	Match(line string, timestamp string) (*models.Event, bool)
}

// ExtractFunc extracts domain-specific metadata (missionID, errorGUID, details) from regex matches.
type ExtractFunc func(matches []string, line string) (missionID string, errorGUID string, details string)

// PatternFilter is a regex-backed implementation of the Filter interface.
type PatternFilter struct {
	name      string
	eventType models.EventType
	severity  models.Severity
	regex     *regexp.Regexp
	extract   ExtractFunc
}

// NewPatternFilter creates an initialized PatternFilter instance.
func NewPatternFilter(name string, eventType models.EventType, severity models.Severity, pattern string, extract ExtractFunc) (*PatternFilter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex for filter %q: %w", name, err)
	}

	return &PatternFilter{
		name:      name,
		eventType: eventType,
		severity:  severity,
		regex:     re,
		extract:   extract,
	}, nil
}

// Name returns the descriptive name of the filter.
func (pf *PatternFilter) Name() string {
	return pf.name
}

// Type returns the models.EventType emitted by this filter.
func (pf *PatternFilter) Type() models.EventType {
	return pf.eventType
}

// Severity returns the models.Severity assigned to events from this filter.
func (pf *PatternFilter) Severity() models.Severity {
	return pf.severity
}

// Match evaluates whether the line satisfies the filter regex and extracts relevant fields.
func (pf *PatternFilter) Match(line string, timestamp string) (*models.Event, bool) {
	matches := pf.regex.FindStringSubmatch(line)
	if len(matches) == 0 {
		return nil, false
	}

	var missionID, errorGUID, details string
	if pf.extract != nil {
		missionID, errorGUID, details = pf.extract(matches, line)
	}

	if details == "" {
		details = strings.TrimSpace(line)
	}

	return &models.Event{
		Timestamp: timestamp,
		Type:      pf.eventType,
		Severity:  pf.severity,
		MissionID: missionID,
		ErrorGUID: errorGUID,
		Details:   details,
		RawLine:   line,
		ParsedAt:  time.Now().UTC(),
	}, true
}

// Registry manages a collection of filters and evaluates them in order against raw log lines.
type Registry struct {
	filters        []Filter
	timestampRegex *regexp.Regexp
}

// NewRegistry constructs an empty Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		filters:        nil,
		timestampRegex: defaultTimestampRegex,
	}
}

// Register adds one or more filters to the registry.
func (r *Registry) Register(filters ...Filter) {
	r.filters = append(r.filters, filters...)
}

// Filters returns a slice containing all registered filters.
func (r *Registry) Filters() []Filter {
	result := make([]Filter, len(r.filters))
	copy(result, r.filters)
	return result
}

// ExtractTimestamp extracts the ISO-8601 timestamp prefix from a log line if present.
func (r *Registry) ExtractTimestamp(line string) string {
	matches := r.timestampRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Evaluate runs the incoming line against registered filters and returns the first matching Event.
func (r *Registry) Evaluate(line string) (*models.Event, bool) {
	ts := r.ExtractTimestamp(line)

	for _, f := range r.filters {
		if event, matched := f.Match(line, ts); matched {
			return event, true
		}
	}

	return nil, false
}

// NewDefaultRegistry creates a fully-populated Registry containing all forensic filters
// specified in docs/draft_contract_filter.md, docs/log_patterns.md, and session state lifecycles.
func NewDefaultRegistry() (*Registry, error) {
	reg := NewRegistry()

	guidExtractor := func(matches []string, line string) (string, string, string) {
		if len(matches) > 1 {
			return matches[1], "", strings.TrimSpace(line)
		}
		return "", "", strings.TrimSpace(line)
	}

	errorGUIDExtractor := func(matches []string, line string) (string, string, string) {
		if len(matches) > 1 {
			return "", matches[1], strings.TrimSpace(line)
		}
		return "", "", strings.TrimSpace(line)
	}

	plainExtractor := func(matches []string, line string) (string, string, string) {
		return "", "", strings.TrimSpace(line)
	}

	statusExtractor := func(matches []string, line string) (string, string, string) {
		detail := strings.TrimSpace(line)
		if len(matches) > 1 {
			detail = fmt.Sprintf("Status: %s", matches[1])
		}
		return "", "", detail
	}

	definitions := []struct {
		name      string
		eventType models.EventType
		severity  models.Severity
		pattern   string
		extract   ExtractFunc
	}{
		// Contract & Mission Events
		{
			name:      "Mission Accepted",
			eventType: models.EventMissionAccepted,
			severity:  models.SeverityInfo,
			pattern:   `Contract Accepted: .* MissionId: \[([a-f0-9\-]{36})\]`,
			extract:   guidExtractor,
		},
		{
			name:      "Mission Completed",
			eventType: models.EventMissionCompleted,
			severity:  models.SeverityInfo,
			pattern:   `Contract Complete: .* MissionId: \[([a-f0-9\-]{36})\]`,
			extract:   guidExtractor,
		},
		{
			name:      "Mission Failed",
			eventType: models.EventMissionFailed,
			severity:  models.SeverityWarning,
			pattern:   `Contract Failed: .* MissionId: \[([a-f0-9\-]{36})\]`,
			extract:   guidExtractor,
		},
		{
			name:      "Mission Abandoned",
			eventType: models.EventMissionAbandoned,
			severity:  models.SeverityWarning,
			pattern:   `Contract Abandoned: .* MissionId: \[([a-f0-9\-]{36})\]`,
			extract:   guidExtractor,
		},
		{
			name:      "Objective Spawned",
			eventType: models.EventObjectiveSpawned,
			severity:  models.SeverityInfo,
			pattern:   `CLocalMissionPhaseMarker::CreateMarker.*missionId \[([a-f0-9\-]{36})\]`,
			extract:   guidExtractor,
		},
		{
			name:      "DGS Failure",
			eventType: models.EventDGSFailure,
			severity:  models.SeverityCritical,
			pattern:   `!!! Failed to register RPC \[RPC\.DGS\..*\]`,
			extract:   plainExtractor,
		},
		{
			name:      "Subsumption Error",
			eventType: models.EventSubsumptionError,
			severity:  models.SeverityError,
			pattern:   `\[Error\] <\[Subsumption::ErrorReporter\]> .* \(GUID:([a-f0-9\-]{36})\)`,
			extract:   errorGUIDExtractor,
		},
		{
			name:      "Subsumption Mismatch",
			eventType: models.EventSubsumptionMismatch,
			severity:  models.SeverityError,
			pattern:   `Failed to create Subsumption (?:task|variable|dependency) .* associated with the guid '([a-f0-9\-]{36})'`,
			extract:   errorGUIDExtractor,
		},
		{
			name:      "Contract Status",
			eventType: models.EventContractStatus,
			severity:  models.SeverityInfo,
			pattern:   `CContractGenerator::CContractGenerator.*(?:completed|failed|closed|Status:\s*\w+)`,
			extract:   statusExtractor,
		},
		{
			name:      "Contract Init",
			eventType: models.EventContractInit,
			severity:  models.SeverityNotice,
			pattern:   `CContractGenerator::CContractGenerator`,
			extract:   plainExtractor,
		},
		{
			name:      "Mission Result",
			eventType: models.EventMissionResult,
			severity:  models.SeverityInfo,
			pattern:   `(?i)(?:Contract|Mission).*?(?:result|outcome)\s+is\s+(Success|Failure|Completed|Failed)`,
			extract: func(matches []string, line string) (string, string, string) {
				detail := strings.TrimSpace(line)
				if len(matches) > 1 {
					detail = fmt.Sprintf("Result: %s", matches[1])
				}
				return "", "", detail
			},
		},

		// Lifecycle State Transition Events
		{
			name:      "Main Menu Loaded",
			eventType: models.EventStateTransition,
			severity:  models.SeverityNotice,
			pattern:   `(?i)(?:RequestFrontEndReason="ProcessBootRequest"|gamerules="SC_Frontend"|CEntityComponentFrontendPUUIProvider::AddAccessibleDestinations)`,
			extract: func(matches []string, line string) (string, string, string) {
				return "", "", "State Transition: Main Menu / FrontEnd"
			},
		},
		{
			name:      "Joining Server Handshake",
			eventType: models.EventStateTransition,
			severity:  models.SeverityNotice,
			pattern:   `(?i)(?:remoteAddr=\d+\.\d+\.\d+\.\d+.*gamerules="SC_Frontend"|CDisciplineServiceExternal::BeginSession|gamerules="SC_Default".*taskname="ModelWaitForViews"|ConnectToServer)`,
			extract: func(matches []string, line string) (string, string, string) {
				return "", "", "State Transition: Joining Server (DGS Handshake)"
			},
		},
		{
			name:      "Gameplay Spawn Complete",
			eventType: models.EventStateTransition,
			severity:  models.SeverityNotice,
			pattern:   `(?i)(?:taskname="StopLoadingScreen".*gamerules="SC_Default"|taskname="OnClientEnteredGame".*gamerules="SC_Default"|Local player spawned|Client entered world)`,
			extract: func(matches []string, line string) (string, string, string) {
				return "", "", "State Transition: Gameplay (In-World)"
			},
		},
		{
			name:      "Server Disconnect / Exit Menu",
			eventType: models.EventServerDisconnect,
			severity:  models.SeverityNotice,
			pattern:   `(?i)(?:Remote Disconnect - Player requested disconnect|EALobbyQuit.*RequestQuitLobby|\[CSessionManager::RequestFrontEnd\] Restored)`,
			extract: func(matches []string, line string) (string, string, string) {
				return "", "", "State Transition: Server Disconnect (Returned to Menu)"
			},
		},
	}

	for _, def := range definitions {
		f, err := NewPatternFilter(def.name, def.eventType, def.severity, def.pattern, def.extract)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize filter %q: %w", def.name, err)
		}
		reg.Register(f)
	}

	return reg, nil
}
