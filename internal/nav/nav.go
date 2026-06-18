// Package nav defines the shared navigation model for Spela TUI and GUI.
// Navigation is organized as Destination (primary) → Scope/Aspect/Section
// (context) → content detail.
package nav

import "strings"

// Destination is a top-level application area (primary nav).
type Destination int

const (
	DestinationLibrary Destination = iota
	DestinationDLLCatalog
	DestinationMonitor
	DestinationSettings
)

// DestinationLabels are human-readable names for breadcrumbs and UI chrome.
var DestinationLabels = []string{
	"Library",
	"DLL Catalog",
	"Monitor",
	"Settings",
}

func (d Destination) String() string {
	if int(d) >= 0 && int(d) < len(DestinationLabels) {
		return DestinationLabels[d]
	}
	return "unknown"
}

// DestinationHotkey returns the digit hotkey for a destination ("1".."4").
func DestinationHotkey(d Destination) string {
	return string(rune('1' + d))
}

// DestinationFromHotkey maps "1".."4" to a destination. The second return is
// false when the key does not select a destination.
func DestinationFromHotkey(key string) (Destination, bool) {
	switch key {
	case "1":
		return DestinationLibrary, true
	case "2":
		return DestinationDLLCatalog, true
	case "3":
		return DestinationMonitor, true
	case "4":
		return DestinationSettings, true
	default:
		return DestinationLibrary, false
	}
}

// Zone is a keyboard-focus region in the shell layout.
type Zone int

const (
	ZonePrimary Zone = iota
	ZoneContext
	ZoneContent
)

// ScopeKind distinguishes global default profile vs a specific game.
type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota
	ScopeGame
)

// Aspect is what the user is viewing within Library scope.
type Aspect int

const (
	AspectOverview Aspect = iota
	AspectProfile
	AspectDLLs
)

var AspectLabels = []string{"Overview", "Profile", "DLLs"}

func (a Aspect) String() string {
	if int(a) >= 0 && int(a) < len(AspectLabels) {
		return AspectLabels[a]
	}
	return "unknown"
}

// DLLCatalogSection is a subsection within the DLL Catalog destination.
type DLLCatalogSection int

const (
	SectionDLLLibrary DLLCatalogSection = iota
	SectionDLLDeployment
)

var DLLCatalogSectionLabels = []string{"Library", "Deployment"}

func (s DLLCatalogSection) String() string {
	if int(s) >= 0 && int(s) < len(DLLCatalogSectionLabels) {
		return DLLCatalogSectionLabels[s]
	}
	return "unknown"
}

// MonitorSection identifies GPU/CPU/Alerts sub-panes.
type MonitorSection int

const (
	MonitorGPU MonitorSection = iota
	MonitorCPU
	MonitorAlerts
)

var MonitorSectionLabels = []string{"GPU", "CPU", "Alerts"}

func (s MonitorSection) String() string {
	if int(s) >= 0 && int(s) < len(MonitorSectionLabels) {
		return MonitorSectionLabels[s]
	}
	return "unknown"
}

// SettingsSection identifies settings groups.
type SettingsSection int

const (
	SettingsDisplay SettingsSection = iota
	SettingsStartup
	SettingsPaths
	SettingsDLLPolicy
	SettingsLogging
)

var SettingsSectionLabels = []string{
	"Display",
	"Startup",
	"Paths",
	"DLL policy",
	"Logging",
}

func (s SettingsSection) String() string {
	if int(s) >= 0 && int(s) < len(SettingsSectionLabels) {
		return SettingsSectionLabels[s]
	}
	return "unknown"
}

// ProfileSubsystem identifies a profile field group (canonical order).
type ProfileSubsystem int

const (
	SubsystemProton ProfileSubsystem = iota
	SubsystemDLSS
	SubsystemGPU
	SubsystemCPU
	SubsystemOverlay
)

var ProfileSubsystemLabels = []string{"Proton", "DLSS", "GPU", "CPU", "Overlay"}

// ProfileSubsystemKeys are the YAML section keys matching internal/profile.
var ProfileSubsystemKeys = []string{"proton", "dlss", "gpu", "cpu", "overlay"}

func (s ProfileSubsystem) String() string {
	if int(s) >= 0 && int(s) < len(ProfileSubsystemLabels) {
		return ProfileSubsystemLabels[s]
	}
	return "unknown"
}

func (s ProfileSubsystem) Key() string {
	if int(s) >= 0 && int(s) < len(ProfileSubsystemKeys) {
		return ProfileSubsystemKeys[s]
	}
	return ""
}

// Scope identifies who configuration applies to within Library.
type Scope struct {
	Kind     ScopeKind
	GameName string
	AppID    uint64
}

// State is the full navigation state shared by TUI and GUI.
type State struct {
	Destination       Destination
	Zone              Zone
	Scope             Scope
	Aspect            Aspect
	ProfileSubsystem  ProfileSubsystem
	DLLCatalogSection DLLCatalogSection
	MonitorSection    MonitorSection
	SettingsSection   SettingsSection
}

// DefaultState returns the initial navigation state (Library, global scope).
func DefaultState() State {
	return State{
		Destination:      DestinationLibrary,
		Zone:             ZonePrimary,
		Scope:            Scope{Kind: ScopeGlobal},
		Aspect:           AspectProfile,
		ProfileSubsystem: SubsystemProton,
	}
}

// SelectDestination switches primary nav and resets context to sensible defaults.
func (s State) SelectDestination(d Destination) State {
	s.Destination = d
	s.Zone = ZonePrimary
	switch d {
	case DestinationLibrary:
		if s.Scope.Kind == ScopeGlobal {
			s.Aspect = AspectProfile
		} else {
			s.Aspect = AspectOverview
		}
	case DestinationDLLCatalog:
		s.DLLCatalogSection = SectionDLLLibrary
	case DestinationMonitor:
		s.MonitorSection = MonitorGPU
	case DestinationSettings:
		s.SettingsSection = SettingsDisplay
	}
	return s
}

// SelectScope updates Library scope and picks a valid aspect.
func (s State) SelectScope(scope Scope) State {
	s.Scope = scope
	if scope.Kind == ScopeGlobal {
		s.Aspect = AspectProfile
		return s
	}
	// Game scope: keep Profile or DLLs; otherwise default to Overview.
	switch s.Aspect {
	case AspectProfile, AspectDLLs:
		// unchanged
	default:
		s.Aspect = AspectOverview
	}
	return s
}

// SelectAspect sets the Library aspect (game scope only for Overview/DLLs).
func (s State) SelectAspect(a Aspect) State {
	if s.Scope.Kind == ScopeGlobal && (a == AspectOverview || a == AspectDLLs) {
		return s
	}
	s.Aspect = a
	return s
}

// SelectProfileSubsystem sets the active profile section.
func (s State) SelectProfileSubsystem(sub ProfileSubsystem) State {
	s.ProfileSubsystem = sub
	return s
}

// Breadcrumb returns the navigation trail segments (without "Spela" root).
func (s State) Breadcrumb() []string {
	var parts []string
	parts = append(parts, s.Destination.String())

	switch s.Destination {
	case DestinationLibrary:
		if s.Scope.Kind == ScopeGlobal {
			parts = append(parts, "All games")
		} else if s.Scope.GameName != "" {
			parts = append(parts, s.Scope.GameName)
		}
		parts = append(parts, s.Aspect.String())
		if s.Aspect == AspectProfile {
			parts = append(parts, s.ProfileSubsystem.String())
		}
	case DestinationDLLCatalog:
		parts = append(parts, s.DLLCatalogSection.String())
	case DestinationMonitor:
		parts = append(parts, s.MonitorSection.String())
	case DestinationSettings:
		parts = append(parts, s.SettingsSection.String())
	}
	return parts
}

// BreadcrumbString joins segments with the canonical separator.
func (s State) BreadcrumbString() string {
	parts := s.Breadcrumb()
	if len(parts) == 0 {
		return "Spela"
	}
	return "Spela › " + strings.Join(parts, " › ")
}

// ContentHints carries TUI content state for DLL-related status-bar hints.
type ContentHints struct {
	HasUpdates   bool
	HasBackup    bool
	DLLOperating bool
}

// ContextKey is one keybinding hint for the status bar.
type ContextKey struct {
	Key    string
	Action string
	Reason string // non-empty when disabled
}

// ContextKeys returns key hints for the current state and focus zone.
func (s State) ContextKeys(showHints bool, hints ContentHints) []ContextKey {
	if !showHints {
		return nil
	}
	switch s.Zone {
	case ZonePrimary:
		return []ContextKey{
			{Key: "1-4", Action: "destination"},
			{Key: "j/k", Action: "navigate"},
			{Key: "Tab", Action: "context"},
			{Key: "q", Action: "quit"},
		}
	case ZoneContext:
		return s.contextZoneKeys()
	case ZoneContent:
		return s.contentZoneKeys(hints)
	}
	return nil
}

func (s State) contextZoneKeys() []ContextKey {
	switch s.Destination {
	case DestinationLibrary:
		keys := []ContextKey{
			{Key: "/", Action: "search"},
			{Key: "Tab", Action: "content"},
			{Key: "Esc", Action: "primary"},
		}
		if s.Scope.Kind == ScopeGame {
			keys = append([]ContextKey{{Key: "1-3", Action: "aspect"}}, keys...)
		}
		return keys
	case DestinationDLLCatalog:
		return []ContextKey{
			{Key: "j/k", Action: "section"},
			{Key: "Tab", Action: "content"},
			{Key: "Esc", Action: "primary"},
		}
	case DestinationMonitor, DestinationSettings:
		return []ContextKey{
			{Key: "j/k", Action: "section"},
			{Key: "Tab", Action: "content"},
			{Key: "Esc", Action: "primary"},
		}
	}
	return nil
}

func (s State) contentZoneKeys(hints ContentHints) []ContextKey {
	switch s.Destination {
	case DestinationLibrary:
		switch s.Aspect {
		case AspectProfile:
			return []ContextKey{
				{Key: "j/k", Action: "field"},
				{Key: "r", Action: "reset"},
				{Key: "p", Action: "pin"},
				{Key: "Shift+R", Action: "reset all"},
				{Key: "Esc", Action: "context"},
			}
		case AspectDLLs:
			keys := []ContextKey{
				{Key: "i", Action: "install"},
				{Key: "Ctrl+Shift+R", Action: "restore"},
				{Key: "Esc", Action: "context"},
			}
			if hints.HasUpdates && !hints.DLLOperating {
				keys = append([]ContextKey{{Key: "u", Action: "update"}}, keys...)
			}
			return keys
		case AspectOverview:
			return []ContextKey{{Key: "Esc", Action: "context"}}
		}
	case DestinationDLLCatalog:
		return []ContextKey{
			{Key: "j/k", Action: "row"},
			{Key: "U", Action: "update all stale"},
			{Key: "Esc", Action: "context"},
		}
	case DestinationMonitor:
		return []ContextKey{{Key: "Esc", Action: "context"}}
	case DestinationSettings:
		return []ContextKey{
			{Key: "j/k", Action: "option"},
			{Key: "Esc", Action: "context"},
		}
	}
	return []ContextKey{{Key: "Esc", Action: "context"}}
}

// NextZone advances focus forward (Primary → Context → Content).
func (s State) NextZone() State {
	switch s.Zone {
	case ZonePrimary:
		s.Zone = ZoneContext
	case ZoneContext:
		s.Zone = ZoneContent
	}
	return s
}

// PrevZone moves focus backward (Content → Context → Primary).
func (s State) PrevZone() State {
	switch s.Zone {
	case ZoneContent:
		s.Zone = ZoneContext
	case ZoneContext:
		s.Zone = ZonePrimary
	}
	return s
}
