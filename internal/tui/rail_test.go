package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestRail_InitialState(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)

	if r.Cursor() != 0 {
		t.Errorf("expected initial cursor 0, got %d", r.Cursor())
	}
	if r.Active() != nav.DestinationLibrary {
		t.Errorf("expected initial active Library, got %v", r.Active())
	}
	if got := len(primaryEntries); got != 4 {
		t.Errorf("expected exactly 4 primary entries, got %d", got)
	}
	want := []nav.Destination{
		nav.DestinationLibrary,
		nav.DestinationDLLCatalog,
		nav.DestinationMonitor,
		nav.DestinationSettings,
	}
	for i, w := range want {
		if primaryEntries[i].destination != w {
			t.Errorf("entry %d: got %v, want %v", i, primaryEntries[i].destination, w)
		}
	}
	wantHotkeys := []string{"1", "2", "3", "4"}
	for i, k := range wantHotkeys {
		if primaryEntries[i].hotkey != k {
			t.Errorf("entry %d hotkey: got %q, want %q", i, primaryEntries[i].hotkey, k)
		}
	}
}

func TestRail_HotkeySelectsResource(t *testing.T) {
	cases := []struct {
		key  string
		want nav.Destination
	}{
		{"1", nav.DestinationLibrary},
		{"2", nav.DestinationDLLCatalog},
		{"3", nav.DestinationMonitor},
		{"4", nav.DestinationSettings},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			styles := NewStyles(DefaultTheme, true)
			r := NewRail(styles)
			r, _, handled := r.Update(keyMsg(tc.key))
			if !handled {
				t.Fatalf("expected key %q to be handled by rail", tc.key)
			}
			if r.Active() != tc.want {
				t.Errorf("after key %q: got active %v, want %v", tc.key, r.Active(), tc.want)
			}
			for i, e := range primaryEntries {
				if e.destination == tc.want && r.Cursor() != i {
					t.Errorf("after key %q: cursor = %d, want %d", tc.key, r.Cursor(), i)
				}
			}
		})
	}
}

func TestRail_JKMovesCursor(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)

	r, _, _ = r.Update(keyMsg("j"))
	if r.Cursor() != 1 {
		t.Errorf("after j: cursor = %d, want 1", r.Cursor())
	}
	if r.Active() != nav.DestinationLibrary {
		t.Errorf("active changed without enter: got %v, want Library", r.Active())
	}

	r, _, _ = r.Update(keyMsg("k"))
	if r.Cursor() != 0 {
		t.Errorf("after k: cursor = %d, want 0", r.Cursor())
	}
}

func TestRail_EnterActivatesCursor(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)
	r, _, _ = r.Update(keyMsg("j"))
	r, _, _ = r.Update(keyMsg("enter"))
	if r.Active() != nav.DestinationDLLCatalog {
		t.Errorf("after enter on row 1: active = %v, want DLL Catalog", r.Active())
	}
}

func TestRail_HotkeyFromDeepFocus(t *testing.T) {
	m := testLayoutWithGame(testGame("Cyberpunk 2077"))
	if m.navState.Zone != nav.ZoneContent {
		t.Fatalf("precondition: expected content zone, got %v", m.navState.Zone)
	}

	result, _ := sendKey(&m, "2")
	layout := result.(LayoutModel)
	if layout.rail.Active() != nav.DestinationLibrary {
		t.Errorf("hotkey outside Primary zone must not change destination, got %v", layout.rail.Active())
	}
	if layout.navState.Zone != nav.ZoneContent {
		t.Errorf("expected content zone unchanged, got %v", layout.navState.Zone)
	}
}

func TestPane_ViewPerDestination(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	pane := newResourcePane(styles, NewContent(styles, false, DefaultServices()))

	cases := []struct {
		dest nav.Destination
		want string
	}{
		{nav.DestinationDLLCatalog, "Library"},
		{nav.DestinationMonitor, "GPU"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			navState := nav.DefaultState().SelectDestination(tc.dest)
			pane.BindNavState(&navState)
			got := pane.View(true)
			if !strings.Contains(got, tc.want) && tc.dest != nav.DestinationDLLCatalog {
				t.Errorf("View(%v): missing %q", tc.dest, tc.want)
			}
		})
	}
}
