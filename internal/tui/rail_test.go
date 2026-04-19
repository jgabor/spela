package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Rail state machine — Task 3 acceptance
// ---------------------------------------------------------------------------

// TestRail_InitialState verifies the rail boots with four entries in the
// canonical order, cursor at 0, ResourceGames active.
func TestRail_InitialState(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)

	if r.Cursor() != 0 {
		t.Errorf("expected initial cursor 0, got %d", r.Cursor())
	}
	if r.Active() != ResourceGames {
		t.Errorf("expected initial active ResourceGames, got %v", r.Active())
	}
	if got := len(railEntries); got != 4 {
		t.Errorf("expected exactly 4 rail entries, got %d", got)
	}
	want := []Resource{ResourceGames, ResourceDLLs, ResourceDefaults, ResourceMetrics}
	for i, w := range want {
		if railEntries[i].resource != w {
			t.Errorf("rail entry %d: got %v, want %v", i, railEntries[i].resource, w)
		}
	}
	wantHotkeys := []string{"1", "2", "3", "4"}
	for i, k := range wantHotkeys {
		if railEntries[i].hotkey != k {
			t.Errorf("rail entry %d hotkey: got %q, want %q", i, railEntries[i].hotkey, k)
		}
	}
}

// TestRail_HotkeySelectsResource verifies each of 1-4 picks the matching
// resource AND moves the cursor to its row. Rail focus is preserved by
// contract: SelectHotkey() itself does not move focus.
func TestRail_HotkeySelectsResource(t *testing.T) {
	cases := []struct {
		key  string
		want Resource
	}{
		{"1", ResourceGames},
		{"2", ResourceDLLs},
		{"3", ResourceDefaults},
		{"4", ResourceMetrics},
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
			// Hotkeys also align the cursor to the selected row.
			for i, e := range railEntries {
				if e.resource == tc.want && r.Cursor() != i {
					t.Errorf("after key %q: cursor = %d, want %d", tc.key, r.Cursor(), i)
				}
			}
		})
	}
}

// TestRail_JKMovesCursor verifies j/k (and arrow aliases) move the cursor
// one step at a time and clamp at the ends. The active resource is NOT
// changed — it only updates on enter (or hotkey).
func TestRail_JKMovesCursor(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)

	// Move down all the way.
	for i := 1; i <= 3; i++ {
		r, _, _ = r.Update(keyMsg("j"))
		if r.Cursor() != i {
			t.Errorf("after %d downs: cursor = %d, want %d", i, r.Cursor(), i)
		}
	}
	// Clamp at end.
	r, _, _ = r.Update(keyMsg("j"))
	if r.Cursor() != 3 {
		t.Errorf("clamp: cursor = %d, want 3", r.Cursor())
	}
	// Active resource must still be ResourceGames (cursor moved, not activated).
	if r.Active() != ResourceGames {
		t.Errorf("active changed without enter: got %v, want ResourceGames", r.Active())
	}

	// Move up all the way.
	for i := 2; i >= 0; i-- {
		r, _, _ = r.Update(keyMsg("k"))
		if r.Cursor() != i {
			t.Errorf("going up: cursor = %d, want %d", r.Cursor(), i)
		}
	}
	// Clamp at start.
	r, _, _ = r.Update(keyMsg("k"))
	if r.Cursor() != 0 {
		t.Errorf("clamp at 0: cursor = %d", r.Cursor())
	}

	// Arrow aliases work too.
	r, _, _ = r.Update(keyMsg("down"))
	if r.Cursor() != 1 {
		t.Errorf("down arrow: cursor = %d, want 1", r.Cursor())
	}
	r, _, _ = r.Update(keyMsg("up"))
	if r.Cursor() != 0 {
		t.Errorf("up arrow: cursor = %d, want 0", r.Cursor())
	}
}

// TestRail_EnterConfirmsCursor verifies that j/k followed by enter sets
// the active resource to the cursor's row — the alternative path to hotkeys.
func TestRail_EnterConfirmsCursor(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	r := NewRail(styles)

	// Move to DLLs row.
	r, _, _ = r.Update(keyMsg("j"))
	if r.Cursor() != 1 {
		t.Fatalf("precondition: cursor should be 1, got %d", r.Cursor())
	}
	if r.Active() != ResourceGames {
		t.Fatalf("precondition: active should still be ResourceGames, got %v", r.Active())
	}
	r, _, _ = r.Update(keyMsg("enter"))
	if r.Active() != ResourceDLLs {
		t.Errorf("after enter: active = %v, want ResourceDLLs", r.Active())
	}
}

// TestRail_HotkeyFromLayoutStaysOnRail verifies the full layout flow —
// pressing 1-4 from any focus state keeps railFocused=true, resets inner
// focus, and swaps the active resource.
func TestRail_HotkeyFromLayoutStaysOnRail(t *testing.T) {
	m := testLayout()
	m.railFocused = false // simulate being deep in a resource
	m.pane.SetInnerFocused(true)

	cases := []struct {
		key  string
		want Resource
	}{
		{"2", ResourceDLLs},
		{"3", ResourceDefaults},
		{"4", ResourceMetrics},
		{"1", ResourceGames},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			result, _ := sendKey(&m, tc.key)
			layout := result.(LayoutModel)
			if !layout.railFocused {
				t.Errorf("expected rail focus after %q", tc.key)
			}
			if layout.rail.Active() != tc.want {
				t.Errorf("after %q: active = %v, want %v", tc.key, layout.rail.Active(), tc.want)
			}
			if layout.pane.InnerFocused() {
				t.Errorf("expected inner focus reset after rail hotkey")
			}
			m = layout // chain
		})
	}
}

// TestRail_RouterRendersResources verifies the resource router dispatches
// to the correct renderer for DLLs and Metrics. As of Task 6 both panes
// have substantive content: the DLLs view always renders its "Library"
// title, and the Metrics view always renders its "Metrics" title.
func TestRail_RouterRendersResources(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	svc := testServices()
	sidebar, _ := NewSidebar(nil, styles, svc)
	content := NewContent(styles, true, svc)
	pane := newResourcePane(styles, sidebar, content)
	pane.setServices(svc)
	pane.SetSize(100, 20)

	cases := []struct {
		resource Resource
		wantSub  string
	}{
		{ResourceDLLs, "Library"},
		{ResourceMetrics, "Metrics"},
	}
	for _, tc := range cases {
		t.Run(tc.resource.String(), func(t *testing.T) {
			got := pane.View(tc.resource, true)
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("render for %v missing %q in output:\n%s", tc.resource, tc.wantSub, got)
			}
		})
	}
}

// TestRail_GamesResourceRendersGamesList verifies that the Games resource
// shows the games sidebar, not a stub.
func TestRail_GamesResourceRendersGamesList(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayout(g)
	if m.rail.Active() != ResourceGames {
		t.Fatalf("expected ResourceGames active initially, got %v", m.rail.Active())
	}
	out := m.pane.View(ResourceGames, true)
	if !strings.Contains(out, "Cyberpunk") {
		t.Errorf("expected games-resource view to contain 'Cyberpunk', got:\n%s", out)
	}
}
