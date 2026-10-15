package nav

import (
	"strings"
	"testing"
)

func TestDefaultState(t *testing.T) {
	s := DefaultState()
	if s.Destination != DestinationLibrary {
		t.Errorf("destination: got %v", s.Destination)
	}
	if s.Scope.Kind != ScopeGlobal {
		t.Errorf("scope: got %v", s.Scope.Kind)
	}
	if s.Aspect != AspectProfile {
		t.Errorf("aspect: got %v", s.Aspect)
	}
}

func TestDestinationFromHotkey(t *testing.T) {
	cases := []struct {
		key  string
		want Destination
		ok   bool
	}{
		{"1", DestinationLibrary, true},
		{"2", DestinationDLLCatalog, true},
		{"3", DestinationMonitor, true},
		{"4", DestinationSettings, true},
		{"5", DestinationLibrary, false},
	}
	for _, tc := range cases {
		got, ok := DestinationFromHotkey(tc.key)
		if ok != tc.ok || got != tc.want {
			t.Errorf("DestinationFromHotkey(%q) = (%v, %v), want (%v, %v)", tc.key, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNavigationLabelsAndBreadcrumbsCoverSupportedStates(t *testing.T) {
	for destination := DestinationLibrary; destination <= DestinationSettings; destination++ {
		if destination.String() == "unknown" || DestinationHotkey(destination) == "" {
			t.Errorf("destination %d label/hotkey missing", destination)
		}
	}
	if Destination(99).String() != "unknown" || Aspect(99).String() != "unknown" || DLLCatalogSection(99).String() != "unknown" || MonitorSection(99).String() != "unknown" || SettingsSection(99).String() != "unknown" || ProfileSubsystem(99).String() != "unknown" || ProfileSubsystem(99).Key() != "" {
		t.Fatal("invalid navigation values did not use unknown/empty fallbacks")
	}
	states := []State{
		DefaultState(),
		DefaultState().SelectScope(Scope{Kind: ScopeGame, GameName: "Cyberpunk", AppID: 1091500}).SelectAspect(AspectOverview),
		DefaultState().SelectScope(Scope{Kind: ScopeGame, GameName: "Cyberpunk", AppID: 1091500}).SelectAspect(AspectProfile),
		DefaultState().SelectScope(Scope{Kind: ScopeGame, GameName: "Cyberpunk", AppID: 1091500}).SelectAspect(AspectDLLs),
		DefaultState().SelectDestination(DestinationDLLCatalog),
		DefaultState().SelectDestination(DestinationMonitor),
		DefaultState().SelectDestination(DestinationSettings),
		{Destination: Destination(99)},
	}
	for _, state := range states {
		if state.Destination != Destination(99) && !strings.Contains(state.BreadcrumbString(), state.Destination.String()) {
			t.Errorf("breadcrumb %q missing destination %q", state.BreadcrumbString(), state.Destination)
		}
	}
}

func TestBreadcrumb_LibraryGameProfile(t *testing.T) {
	s := DefaultState()
	s.Scope = Scope{Kind: ScopeGame, GameName: "Cyberpunk 2077", AppID: 1091500}
	s.Aspect = AspectProfile
	s.ProfileSubsystem = SubsystemDLSS

	got := s.Breadcrumb()
	want := []string{"Library", "Cyberpunk 2077", "Profile", "DLSS"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("breadcrumb[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
	if !strings.Contains(s.BreadcrumbString(), "Cyberpunk 2077") {
		t.Errorf("BreadcrumbString: %q", s.BreadcrumbString())
	}
}

func TestBreadcrumb_GlobalDefault(t *testing.T) {
	s := DefaultState()
	s.Scope = Scope{Kind: ScopeGlobal}
	s.Aspect = AspectProfile
	s.ProfileSubsystem = SubsystemGPU

	got := s.Breadcrumb()
	if got[1] != "All games" {
		t.Errorf("global scope label: got %q", got[1])
	}
}

func TestBreadcrumb_DLLCatalog(t *testing.T) {
	s := DefaultState().SelectDestination(DestinationDLLCatalog)
	s.DLLCatalogSection = SectionDLLDeployment
	got := s.Breadcrumb()
	if got[len(got)-1] != "Deployment" {
		t.Errorf("got %v", got)
	}
}

func TestSelectDestination_GameScopeKeepsDLLsAndOtherwiseOpensProfile(t *testing.T) {
	for _, test := range []struct{ before, want Aspect }{{AspectDLLs, AspectDLLs}, {AspectProfile, AspectProfile}, {AspectOverview, AspectProfile}} {
		s := DefaultState()
		s.Scope = Scope{Kind: ScopeGame, GameName: "Cyberpunk 2077", AppID: 1091500}
		s.Aspect = test.before
		s = s.SelectDestination(DestinationMonitor)
		s = s.SelectDestination(DestinationLibrary)
		if s.Aspect != test.want {
			t.Errorf("SelectDestination(Library) with game scope from %v: got %v, want %v", test.before, s.Aspect, test.want)
		}
	}
}

func TestSelectAspect_GlobalBlocksOverview(t *testing.T) {
	s := DefaultState()
	s.Scope = Scope{Kind: ScopeGlobal}
	s.Aspect = AspectProfile

	next := s.SelectAspect(AspectOverview)
	if next.Aspect != AspectProfile {
		t.Errorf("global scope should not switch to overview, got %v", next.Aspect)
	}
}

func TestSelectScope_GameOpensProfileAndKeepsDLLs(t *testing.T) {
	s := DefaultState()
	s = s.SelectScope(Scope{Kind: ScopeGame, GameName: "Test"})
	if s.Aspect != AspectProfile {
		t.Errorf("a game from All games opened on %v, want Profile", s.Aspect)
	}
	s = s.SelectAspect(AspectDLLs).SelectScope(Scope{Kind: ScopeGame, GameName: "Other"})
	if s.Aspect != AspectDLLs {
		t.Errorf("the next game opened on %v, want DLLs kept", s.Aspect)
	}
	s.Aspect = AspectOverview
	if got := s.SelectScope(Scope{Kind: ScopeGame, GameName: "Third"}).Aspect; got != AspectProfile {
		t.Errorf("an Overview from older state opened as %v, want Profile", got)
	}
	if got := s.SelectScope(Scope{Kind: ScopeGlobal}).Aspect; got != AspectProfile {
		t.Errorf("All games opened on %v, want Profile", got)
	}
}

func TestProfileSubsystemKey(t *testing.T) {
	if SubsystemDLSS.Key() != "dlss" {
		t.Errorf("got %q", SubsystemDLSS.Key())
	}
}
