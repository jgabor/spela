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

func TestSelectDestination_GameScopeResetsOverview(t *testing.T) {
	s := DefaultState()
	s.Scope = Scope{Kind: ScopeGame, GameName: "Cyberpunk 2077", AppID: 1091500}
	s.Aspect = AspectDLLs
	s = s.SelectDestination(DestinationMonitor)
	s = s.SelectDestination(DestinationLibrary)
	if s.Aspect != AspectOverview {
		t.Errorf("SelectDestination(Library) with game scope: got %v, want Overview", s.Aspect)
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

func TestSelectScope_GameDefaultsOverview(t *testing.T) {
	s := DefaultState()
	s.Scope = Scope{Kind: ScopeGame, GameName: "Test"}
	s = s.SelectScope(Scope{Kind: ScopeGame, GameName: "Test"})
	if s.Aspect != AspectOverview && s.Aspect != AspectProfile {
		t.Errorf("unexpected aspect after game scope: %v", s.Aspect)
	}
}

func TestContextKeys_ZonePrimary(t *testing.T) {
	s := DefaultState()
	s.Zone = ZonePrimary
	keys := s.ContextKeys(true, ContentHints{})
	if len(keys) == 0 {
		t.Fatal("expected keys")
	}
	if keys[0].Key != "1-4" {
		t.Errorf("first key: got %q", keys[0].Key)
	}
}

func TestContextKeys_HintsDisabled(t *testing.T) {
	s := DefaultState()
	if got := s.ContextKeys(false, ContentHints{}); len(got) != 0 {
		t.Errorf("expected no keys when hints disabled")
	}
}

func TestNextPrevZone(t *testing.T) {
	s := DefaultState()
	if s.Zone != ZonePrimary {
		t.Fatal("precondition")
	}
	s = s.NextZone()
	if s.Zone != ZoneContext {
		t.Errorf("after NextZone: %v", s.Zone)
	}
	s = s.NextZone()
	if s.Zone != ZoneContent {
		t.Errorf("after second NextZone: %v", s.Zone)
	}
	s = s.PrevZone()
	if s.Zone != ZoneContext {
		t.Errorf("after PrevZone: %v", s.Zone)
	}
}

func TestProfileSubsystemKey(t *testing.T) {
	if SubsystemDLSS.Key() != "dlss" {
		t.Errorf("got %q", SubsystemDLSS.Key())
	}
}

func TestContextKeys_ContentDLLs_OmitsUpdateWhenUpToDate(t *testing.T) {
	s := DefaultState()
	s.Zone = ZoneContent
	s.Scope = Scope{Kind: ScopeGame, GameName: "Test"}
	s.Aspect = AspectDLLs
	keys := s.ContextKeys(true, ContentHints{HasUpdates: false})
	for _, k := range keys {
		if k.Key == "u" {
			t.Fatal("expected no update key when DLLs are up to date")
		}
	}
}

func TestContextKeys_ContentDLLs_ShowsUpdateWhenStale(t *testing.T) {
	s := DefaultState()
	s.Zone = ZoneContent
	s.Scope = Scope{Kind: ScopeGame, GameName: "Test"}
	s.Aspect = AspectDLLs
	keys := s.ContextKeys(true, ContentHints{HasUpdates: true})
	found := false
	for _, k := range keys {
		if k.Key == "u" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected update key when updates are available")
	}
}
