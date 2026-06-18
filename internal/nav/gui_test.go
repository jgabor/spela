package nav

import "testing"

func TestDefaultGUIStateMatchesDefaultState(t *testing.T) {
	got := DefaultGUIState()
	if got.Destination != int(DestinationLibrary) {
		t.Fatalf("destination = %d, want %d", got.Destination, DestinationLibrary)
	}
	if !got.ScopeGlobal {
		t.Fatal("expected global scope")
	}
	if got.Aspect != int(AspectProfile) {
		t.Fatalf("aspect = %d, want %d", got.Aspect, AspectProfile)
	}
	if len(got.Breadcrumb) == 0 {
		t.Fatal("expected breadcrumb")
	}
}

func TestSelectDestinationGUI_ResetsDLLCatalogSection(t *testing.T) {
	state := DefaultGUIState()
	state.DLLSection = 1
	next := SelectDestinationGUI(state, int(DestinationDLLCatalog))
	if next.Destination != int(DestinationDLLCatalog) {
		t.Fatalf("destination = %d", next.Destination)
	}
	if next.DLLSection != int(SectionDLLLibrary) {
		t.Fatalf("dllSection = %d, want %d", next.DLLSection, SectionDLLLibrary)
	}
}

func TestSelectDestinationGUI_GameScopeResetsOverview(t *testing.T) {
	state := DefaultGUIState()
	state.ScopeGlobal = false
	state.GameName = "Cyberpunk 2077"
	state.Aspect = int(AspectDLLs)
	state = SelectDestinationGUI(state, int(DestinationMonitor))
	state = SelectDestinationGUI(state, int(DestinationLibrary))
	if state.Aspect != int(AspectOverview) {
		t.Fatalf("aspect = %d, want %d", state.Aspect, AspectOverview)
	}
}

func TestSelectScopeGUI_GlobalForcesProfile(t *testing.T) {
	state := DefaultGUIState()
	state.ScopeGlobal = false
	state.Aspect = int(AspectDLLs)
	next := SelectScopeGUI(state, true, "")
	if !next.ScopeGlobal {
		t.Fatal("expected global scope")
	}
	if next.Aspect != int(AspectProfile) {
		t.Fatalf("aspect = %d, want %d", next.Aspect, AspectProfile)
	}
}

func TestSelectAspectGUI_GlobalBlocksOverview(t *testing.T) {
	state := DefaultGUIState()
	next := SelectAspectGUI(state, int(AspectOverview))
	if next.Aspect != int(AspectProfile) {
		t.Fatalf("aspect = %d, want %d", next.Aspect, AspectProfile)
	}
}

func TestDestinationFromHotkeyGUI(t *testing.T) {
	dest, ok := DestinationFromHotkeyGUI("2")
	if !ok || dest != int(DestinationDLLCatalog) {
		t.Fatalf("got (%d, %v)", dest, ok)
	}
	if _, ok := DestinationFromHotkeyGUI("9"); ok {
		t.Fatal("expected false for unknown hotkey")
	}
}
