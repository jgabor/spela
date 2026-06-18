package nav

// GUIState is the Wails-facing navigation snapshot for the GUI frontend.
type GUIState struct {
	Destination     int      `json:"destination"`
	ScopeGlobal     bool     `json:"scopeGlobal"`
	GameName        string   `json:"gameName"`
	Aspect          int      `json:"aspect"`
	Subsystem       int      `json:"subsystem"`
	DLLSection      int      `json:"dllSection"`
	MonitorSection  int      `json:"monitorSection"`
	SettingsSection int      `json:"settingsSection"`
	Breadcrumb      []string `json:"breadcrumb"`
}

// GUIContract exposes canonical nav constants and labels to the GUI frontend.
type GUIContract struct {
	Destination             map[string]int `json:"destination"`
	Aspect                  map[string]int `json:"aspect"`
	DestinationLabels       []string       `json:"destinationLabels"`
	AspectLabels            []string       `json:"aspectLabels"`
	ProfileSubsystemLabels  []string       `json:"profileSubsystemLabels"`
	SettingsSectionLabels   []string       `json:"settingsSectionLabels"`
	MonitorSectionLabels    []string       `json:"monitorSectionLabels"`
	DLLCatalogSectionLabels []string       `json:"dllCatalogSectionLabels"`
}

// WailsContract returns nav constants and labels for GUI binding.
func WailsContract() GUIContract {
	return GUIContract{
		Destination: map[string]int{
			"Library":    int(DestinationLibrary),
			"DLLCatalog": int(DestinationDLLCatalog),
			"Monitor":    int(DestinationMonitor),
			"Settings":   int(DestinationSettings),
		},
		Aspect: map[string]int{
			"Overview": int(AspectOverview),
			"Profile":  int(AspectProfile),
			"DLLs":     int(AspectDLLs),
		},
		DestinationLabels:       append([]string(nil), DestinationLabels...),
		AspectLabels:            append([]string(nil), AspectLabels...),
		ProfileSubsystemLabels:  append([]string(nil), ProfileSubsystemLabels...),
		SettingsSectionLabels:   append([]string(nil), SettingsSectionLabels...),
		MonitorSectionLabels:    append([]string(nil), MonitorSectionLabels...),
		DLLCatalogSectionLabels: append([]string(nil), DLLCatalogSectionLabels...),
	}
}

// DefaultGUIState returns the initial GUI navigation state.
func DefaultGUIState() GUIState {
	return withBreadcrumb(DefaultState())
}

func withBreadcrumb(s State) GUIState {
	g := toGUIState(s)
	g.Breadcrumb = s.Breadcrumb()
	return g
}

func toGUIState(s State) GUIState {
	return GUIState{
		Destination:     int(s.Destination),
		ScopeGlobal:     s.Scope.Kind == ScopeGlobal,
		GameName:        s.Scope.GameName,
		Aspect:          int(s.Aspect),
		Subsystem:       int(s.ProfileSubsystem),
		DLLSection:      int(s.DLLCatalogSection),
		MonitorSection:  int(s.MonitorSection),
		SettingsSection: int(s.SettingsSection),
	}
}

func fromGUIState(g GUIState) State {
	kind := ScopeGame
	if g.ScopeGlobal {
		kind = ScopeGlobal
	}
	return State{
		Destination:       Destination(g.Destination),
		Scope:             Scope{Kind: kind, GameName: g.GameName},
		Aspect:            Aspect(g.Aspect),
		ProfileSubsystem:  ProfileSubsystem(g.Subsystem),
		DLLCatalogSection: DLLCatalogSection(g.DLLSection),
		MonitorSection:    MonitorSection(g.MonitorSection),
		SettingsSection:   SettingsSection(g.SettingsSection),
	}
}

// SelectDestinationGUI switches primary nav using Go transition rules.
func SelectDestinationGUI(g GUIState, destination int) GUIState {
	return withBreadcrumb(fromGUIState(g).SelectDestination(Destination(destination)))
}

// SelectScopeGUI updates Library scope using Go transition rules.
func SelectScopeGUI(g GUIState, scopeGlobal bool, gameName string) GUIState {
	scope := Scope{Kind: ScopeGame, GameName: gameName}
	if scopeGlobal {
		scope = Scope{Kind: ScopeGlobal}
	}
	return withBreadcrumb(fromGUIState(g).SelectScope(scope))
}

// SelectAspectGUI sets the Library aspect using Go transition rules.
func SelectAspectGUI(g GUIState, aspect int) GUIState {
	return withBreadcrumb(fromGUIState(g).SelectAspect(Aspect(aspect)))
}

// SelectSubsystemGUI sets the active profile subsystem.
func SelectSubsystemGUI(g GUIState, subsystem int) GUIState {
	s := fromGUIState(g)
	s.ProfileSubsystem = ProfileSubsystem(subsystem)
	return withBreadcrumb(s)
}

// SelectDLLSectionGUI sets the DLL catalog section.
func SelectDLLSectionGUI(g GUIState, section int) GUIState {
	s := fromGUIState(g)
	s.DLLCatalogSection = DLLCatalogSection(section)
	return withBreadcrumb(s)
}

// SelectMonitorSectionGUI sets the monitor section.
func SelectMonitorSectionGUI(g GUIState, section int) GUIState {
	s := fromGUIState(g)
	s.MonitorSection = MonitorSection(section)
	return withBreadcrumb(s)
}

// SelectSettingsSectionGUI sets the settings section.
func SelectSettingsSectionGUI(g GUIState, section int) GUIState {
	s := fromGUIState(g)
	s.SettingsSection = SettingsSection(section)
	return withBreadcrumb(s)
}

// DestinationFromHotkeyGUI maps digit hotkeys to destinations for the GUI.
func DestinationFromHotkeyGUI(key string) (int, bool) {
	dest, ok := DestinationFromHotkey(key)
	return int(dest), ok
}
