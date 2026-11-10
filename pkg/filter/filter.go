package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"StarCitizenLogWatcher/pkg/models"
)

var defaultTimestampRegex = regexp.MustCompile(`^<(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{3})?Z?)>`)

// FilterConfig represents a single user-editable filter definition in filters.json.
type FilterConfig struct {
	Name                string              `json:"name"`
	Enabled             bool                `json:"enabled"`
	EventType           models.EventType    `json:"event_type"`
	Severity            models.Severity     `json:"severity"`
	Pattern             string              `json:"pattern"`
	ExtractType         string              `json:"extract_type,omitempty"`
	TargetState         models.SessionState `json:"target_state,omitempty"`
	Meaning             string              `json:"meaning"`
	ActiveInvestigation bool                `json:"active_investigation"`
}

// Filter defines the interface for evaluating raw log lines against specific signatures.
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
	name                string
	eventType           models.EventType
	severity            models.Severity
	regex               *regexp.Regexp
	extract             ExtractFunc
	meaning             string
	targetState         models.SessionState
	activeInvestigation bool
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
		meaning:   name,
	}, nil
}

// NewPatternFilterFromConfig constructs a PatternFilter from a declarative FilterConfig.
func NewPatternFilterFromConfig(cfg FilterConfig) (*PatternFilter, error) {
	extract := buildExtractor(cfg.ExtractType, cfg.Meaning)
	pf, err := NewPatternFilter(cfg.Name, cfg.EventType, cfg.Severity, cfg.Pattern, extract)
	if err != nil {
		return nil, err
	}
	if cfg.Meaning != "" {
		pf.meaning = cfg.Meaning
	}
	pf.targetState = cfg.TargetState
	pf.activeInvestigation = cfg.ActiveInvestigation
	return pf, nil
}

func buildExtractor(extractType string, meaning string) ExtractFunc {
	switch strings.ToLower(extractType) {
	case "mission_id":
		return func(matches []string, line string) (string, string, string) {
			for i := 1; i < len(matches); i++ {
				if matches[i] != "" {
					return matches[i], "", strings.TrimSpace(line)
				}
			}
			return "", "", strings.TrimSpace(line)
		}
	case "error_guid":
		return func(matches []string, line string) (string, string, string) {
			if len(matches) > 1 {
				return "", matches[1], strings.TrimSpace(line)
			}
			return "", "", strings.TrimSpace(line)
		}
	case "status":
		return func(matches []string, line string) (string, string, string) {
			detail := strings.TrimSpace(line)
			if len(matches) > 1 {
				detail = fmt.Sprintf("Status: %s", matches[1])
			}
			return "", "", detail
		}
	case "result":
		return func(matches []string, line string) (string, string, string) {
			detail := strings.TrimSpace(line)
			if len(matches) > 1 {
				detail = fmt.Sprintf("Result: %s", matches[1])
			}
			return "", "", detail
		}
	default:
		return func(matches []string, line string) (string, string, string) {
			if meaning != "" {
				return "", "", meaning
			}
			return "", "", strings.TrimSpace(line)
		}
	}
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

	plainMeaning := pf.meaning
	if plainMeaning == "" {
		plainMeaning = string(pf.eventType)
	}
	if missionID != "" {
		plainMeaning = fmt.Sprintf("%s [Mission: %s]", plainMeaning, missionID)
	} else if errorGUID != "" {
		plainMeaning = fmt.Sprintf("%s [GUID: %s]", plainMeaning, errorGUID)
	} else if len(matches) > 1 && matches[1] != "" {
		plainMeaning = fmt.Sprintf("%s (%s)", plainMeaning, matches[1])
	}

	return &models.Event{
		Timestamp:           timestamp,
		Type:                pf.eventType,
		Severity:            pf.severity,
		MissionID:           missionID,
		ErrorGUID:           errorGUID,
		Meaning:             plainMeaning,
		Details:             details,
		RawLine:             line,
		ParsedAt:            time.Now().UTC(),
		TargetState:         pf.targetState,
		ActiveInvestigation: pf.activeInvestigation,
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

// DefaultFilterConfigs returns the default list of filter definitions.
func DefaultFilterConfigs() []FilterConfig {
	return []FilterConfig{
		{
			Name:                "Mission Accepted",
			Enabled:             true,
			EventType:           models.EventMissionAccepted,
			Severity:            models.SeverityInfo,
			Pattern:             `Contract Accepted: .* MissionId: \[([a-f0-9\-]{36})\]`,
			ExtractType:         "mission_id",
			TargetState:         models.SessionStateGameplay,
			Meaning:             "Player accepted a contract",
			ActiveInvestigation: false,
		},
		{
			Name:                "Mission Completed",
			Enabled:             true,
			EventType:           models.EventMissionCompleted,
			Severity:            models.SeverityInfo,
			Pattern:             `(?:Contract Complete: .* MissionId: \[([a-f0-9\-]{36})\]|<EndMission>.*MissionId\[([a-f0-9\-]{36})\].*CompletionType\[Complete\])`,
			ExtractType:         "mission_id",
			Meaning:             "Contract completed successfully",
			ActiveInvestigation: false,
		},
		{
			Name:                "Mission Failed",
			Enabled:             true,
			EventType:           models.EventMissionFailed,
			Severity:            models.SeverityWarning,
			Pattern:             `(?:Contract Failed: .* MissionId: \[([a-f0-9\-]{36})\]|<EndMission>.*MissionId\[([a-f0-9\-]{36})\].*CompletionType\[Fail\w*\])`,
			ExtractType:         "mission_id",
			Meaning:             "Contract failed",
			ActiveInvestigation: false,
		},
		{
			Name:                "Mission Abandoned",
			Enabled:             true,
			EventType:           models.EventMissionAbandoned,
			Severity:            models.SeverityWarning,
			Pattern:             `(?:Contract Abandoned: .* MissionId: \[([a-f0-9\-]{36})\]|<EndMission>.*MissionId\[([a-f0-9\-]{36})\].*CompletionType\[Abandon\])`,
			ExtractType:         "mission_id",
			Meaning:             "Contract abandoned by player",
			ActiveInvestigation: false,
		},
		{
			Name:                "Objective Spawned",
			Enabled:             true,
			EventType:           models.EventObjectiveSpawned,
			Severity:            models.SeverityInfo,
			Pattern:             `CLocalMissionPhaseMarker::CreateMarker.*missionId \[([a-f0-9\-]{36})\]`,
			ExtractType:         "mission_id",
			TargetState:         models.SessionStateGameplay,
			Meaning:             "Mission objective marker spawned in world",
			ActiveInvestigation: false,
		},
		{
			Name:                "Missing Player ObjectDataBank",
			Enabled:             true,
			EventType:           models.EventMissingDataBank,
			Severity:            models.SeverityCritical,
			Pattern:             `<CObjectiveMarkerComponent::(?:AddTo|RemoveFrom)PlayerDataBank> (MissionObjectiveMarker_\d+\[\d+\]) - Player: .* has no ObjectDataBank`,
			ExtractType:         "none",
			Meaning:             "ACTIVE INVESTIGATION: Player entity has no ObjectDataBank for mission objective marker",
			ActiveInvestigation: true,
		},
		{
			Name:                "Objective Marker DataBank Added",
			Enabled:             true,
			EventType:           models.EventMarkerDataBankAdded,
			Severity:            models.SeverityInfo,
			Pattern:             `<CObjectiveMarkerComponent::AddToPlayerDataBank> (MissionObjectiveMarker_\d+\[\d+\]) - Added to DataBank`,
			ExtractType:         "none",
			Meaning:             "Objective marker bound to player DataBank",
			ActiveInvestigation: false,
		},
		{
			Name:                "Objective Marker DataBank Removed",
			Enabled:             true,
			EventType:           models.EventMarkerDataBankRemoved,
			Severity:            models.SeverityWarning,
			Pattern:             `<CObjectiveMarkerComponent::RemoveFromPlayerDataBank> (MissionObjectiveMarker_\d+\[\d+\]) - Removed from DataBank`,
			ExtractType:         "none",
			Meaning:             "Objective marker unbound from player DataBank",
			ActiveInvestigation: false,
		},
		{
			Name:                "Proximity Loss ('Return to' Loop)",
			Enabled:             true,
			EventType:           models.EventProximityLoss,
			Severity:            models.SeverityCritical,
			Pattern:             `<SHUDEvent_OnNotification> Added notification "New Objective: Return to [^"]+".*MissionId: \[([a-f0-9\-]{36})\]`,
			ExtractType:         "mission_id",
			Meaning:             "ACTIVE INVESTIGATION: Mission lost player proximity and triggered 'Return to' local objective loop",
			ActiveInvestigation: true,
		},
		{
			Name:                "Collector Contract Generator Storm",
			Enabled:             true,
			EventType:           models.EventLocationStorm,
			Severity:            models.SeverityWarning,
			Pattern:             `<GenerateLocationProperty> Generated Locations .* contract: (TheCollector_\S+)`,
			ExtractType:         "none",
			Meaning:             "Wikelo / Collector contract generator location spam (server/reputation noise)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Login Identity Metadata Fault",
			Enabled:             true,
			EventType:           models.EventIdentityFault,
			Severity:            models.SeverityWarning,
			Pattern:             `<GetSubscriptionKey Failed>.*No player Id was found in metadata`,
			ExtractType:         "none",
			Meaning:             "Login metadata missing player ID (GetSubscriptionKey UNAUTHENTICATED)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Hyperion Engineering Wing Objective",
			Enabled:             true,
			EventType:           models.EventHyperionStage,
			Severity:            models.SeverityInfo,
			Pattern:             `<SHUDEvent_OnNotification> Added notification "New Objective: Retrieve One of the Reagent Codes in Engineering Wing: ".*MissionId: \[([a-f0-9\-]{36})\]`,
			ExtractType:         "mission_id",
			Meaning:             "Hyperion: Arrived at ASD Onyx Facility (Engineering Wing objective active)",
			ActiveInvestigation: true,
		},
		{
			Name:                "Hyperion Interior Triggered",
			Enabled:             true,
			EventType:           models.EventHyperionStage,
			Severity:            models.SeverityInfo,
			Pattern:             `<SHUDEvent_OnNotification> Added notification "New Objective: Obtain Thermal (?:Vent|Condenser) Diagnostic Data.*MissionId: \[([a-f0-9\-]{36})\]`,
			ExtractType:         "mission_id",
			Meaning:             "Hyperion: Engineering Wing interior Subsumption trigger fired (Healthy)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Deferred Explosion RayCast (Worm Spawn)",
			Enabled:             true,
			EventType:           models.EventDeferredExplosion,
			Severity:            models.SeverityInfo,
			Pattern:             `CDeferredExplosionEffect::RequestRayCast has no grid set, defaulting to global grid`,
			ExtractType:         "none",
			Meaning:             "Deferred Explosion RayCast (Hyperion bridge worm spawn / encounter trigger)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Application Launch / Log Header",
			Enabled:             true,
			EventType:           models.EventStateTransition,
			Severity:            models.SeverityNotice,
			Pattern:             `BackupNameAttachment="\s*([^"]+)"`,
			TargetState:         models.SessionStateAppLaunch,
			Meaning:             "Application Launch",
			ActiveInvestigation: false,
		},
		{
			Name:                "DGS Failure",
			Enabled:             true,
			EventType:           models.EventDGSFailure,
			Severity:            models.SeverityCritical,
			Pattern:             `!!! Failed to register RPC \[RPC\.DGS\..*\]`,
			ExtractType:         "none",
			Meaning:             "DGS RPC registration failed",
			ActiveInvestigation: false,
		},
		{
			Name:                "Subsumption Error",
			Enabled:             true,
			EventType:           models.EventSubsumptionError,
			Severity:            models.SeverityError,
			Pattern:             `\[Error\] <\[Subsumption::ErrorReporter\]> .* \(GUID:([a-f0-9\-]{36})\)`,
			ExtractType:         "error_guid",
			Meaning:             "Subsumption ErrorReporter logged a GUID issue",
			ActiveInvestigation: false,
		},
		{
			Name:                "Subsumption Mismatch",
			Enabled:             true,
			EventType:           models.EventSubsumptionMismatch,
			Severity:            models.SeverityError,
			Pattern:             `Failed to create Subsumption (?:task|variable|dependency) .* associated with the guid '([a-f0-9\-]{36})'`,
			ExtractType:         "error_guid",
			Meaning:             "Subsumption missing archetype or dependency GUID",
			ActiveInvestigation: false,
		},
		{
			Name:                "Contract Status",
			Enabled:             true,
			EventType:           models.EventContractStatus,
			Severity:            models.SeverityInfo,
			Pattern:             `CContractGenerator::CContractGenerator.*(completed|failed|closed|Status:\s*\w+)`,
			ExtractType:         "status",
			Meaning:             "Contract generator updated contract status",
			ActiveInvestigation: false,
		},
		{
			Name:                "Contract Init",
			Enabled:             true,
			EventType:           models.EventContractInit,
			Severity:            models.SeverityNotice,
			Pattern:             `CContractGenerator::CContractGenerator`,
			ExtractType:         "none",
			Meaning:             "Contract generator subsystem initialized",
			ActiveInvestigation: false,
		},
		{
			Name:                "Mission Result",
			Enabled:             true,
			EventType:           models.EventMissionResult,
			Severity:            models.SeverityInfo,
			Pattern:             `(?i)(?:Contract|Mission).*?(?:result|outcome)\s+is\s+(Success|Failure|Completed|Failed)`,
			ExtractType:         "result",
			Meaning:             "Engine validated mission outcome",
			ActiveInvestigation: false,
		},
		{
			Name:                "Main Menu Loaded",
			Enabled:             true,
			EventType:           models.EventStateTransition,
			Severity:            models.SeverityNotice,
			Pattern:             `(?i)(?:RequestFrontEndReason="ProcessBootRequest"|Requesting game mode Frontend_Main/SC_Frontend|CEntityComponentFrontendPUUIProvider::AddAccessibleDestinations)`,
			TargetState:         models.SessionStateMenu,
			Meaning:             "Main Menu loaded",
			ActiveInvestigation: false,
		},
		{
			Name:                "Joining Server Handshake",
			Enabled:             true,
			EventType:           models.EventStateTransition,
			Severity:            models.SeverityNotice,
			Pattern:             `(?i)(?:remoteAddr=\d+\.\d+\.\d+\.\d+.*gamerules="SC_Frontend"|CDisciplineServiceExternal::BeginSession|gamerules="SC_Default".*taskname="ModelWaitForViews"|ConnectToServer)`,
			TargetState:         models.SessionStateJoiningServer,
			Meaning:             "Connecting to game server (DGS handshake)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Gameplay Spawn Complete",
			Enabled:             true,
			EventType:           models.EventStateTransition,
			Severity:            models.SeverityNotice,
			Pattern:             `(?i)(?:taskname="StopLoadingScreen".*gamerules="SC_Default"|taskname="OnClientEnteredGame".*gamerules="SC_Default"|Local player spawned|Client entered world)`,
			TargetState:         models.SessionStateGameplay,
			Meaning:             "Entered game world (Gameplay active)",
			ActiveInvestigation: false,
		},
		{
			Name:                "Server Disconnect / Exit Menu",
			Enabled:             true,
			EventType:           models.EventServerDisconnect,
			Severity:            models.SeverityNotice,
			Pattern:             `(?i)(?:Remote Disconnect - Player requested disconnect|EALobbyQuit.*RequestQuitLobby|\[CSessionManager::RequestFrontEnd\] Restored)`,
			TargetState:         models.SessionStateMenu,
			Meaning:             "Disconnected from server (returned to Main Menu)",
			ActiveInvestigation: false,
		},
	}
}

// NewRegistryFromConfigs compiles and registers all enabled FilterConfigs.
func NewRegistryFromConfigs(configs []FilterConfig) (*Registry, error) {
	reg := NewRegistry()
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		pf, err := NewPatternFilterFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize filter %q: %w", cfg.Name, err)
		}
		reg.Register(pf)
	}
	return reg, nil
}

// NewDefaultRegistry creates a Registry populated with DefaultFilterConfigs.
func NewDefaultRegistry() (*Registry, error) {
	return NewRegistryFromConfigs(DefaultFilterConfigs())
}

// SaveFilterConfigs writes filter configurations as formatted JSON to the specified file path.
func SaveFilterConfigs(path string, configs []FilterConfig) error {
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize filter configs: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write filter config file %s: %w", path, err)
	}
	return nil
}

// LoadRegistryFromFile reads a JSON filter configuration file and returns a compiled Registry.
func LoadRegistryFromFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read filter config file %s: %w", path, err)
	}
	var configs []FilterConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse filter config JSON in %s: %w", path, err)
	}
	return NewRegistryFromConfigs(configs)
}

// LoadOrCreateRegistry loads filters from path if it exists, or creates a default filters.json file if missing.
func LoadOrCreateRegistry(path string) (*Registry, error) {
	if path == "" {
		return NewDefaultRegistry()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		defaults := DefaultFilterConfigs()
		if saveErr := SaveFilterConfigs(path, defaults); saveErr != nil {
			return nil, saveErr
		}
		return NewRegistryFromConfigs(defaults)
	}
	return LoadRegistryFromFile(path)
}
