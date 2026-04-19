package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

// ---------------------------------------------------------------------------
// Disabled field contract — structural iteration
// ---------------------------------------------------------------------------

func TestProfileWidget_DisabledFields_Structural(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	for gi, group := range m.groups {
		for fi, field := range group.fields {
			if !field.disabled {
				continue
			}
			t.Run(field.key+"_contract", func(t *testing.T) {
				// 1. Navigation skips: enter editing on this group, verify
				//    the focused field is not the disabled one (if there's
				//    an enabled field before it).
				w := m
				w.focusedGroup = gi
				w.editing = true
				w.focusedField = fi

				// Try navigating down — should skip this field
				result, _ := w.Update(keyMsg("down"))
				if result.focusedField == fi && hasEnabledFieldAfter(group, fi) {
					t.Errorf("down navigation should skip disabled field %q", field.key)
				}

				// Try navigating up — should skip this field
				w.focusedField = fi
				result, _ = w.Update(keyMsg("up"))
				if result.focusedField == fi && hasEnabledFieldBefore(group, fi) {
					t.Errorf("up navigation should skip disabled field %q", field.key)
				}

				// 2. Input rejected: left/right should not change value
				w.focusedField = fi
				valueBefore := field.value
				result, _ = w.Update(keyMsg("right"))
				if result.groups[gi].fields[fi].value != valueBefore {
					t.Errorf("right should not change disabled field %q value", field.key)
				}

				result, _ = w.Update(keyMsg("left"))
				if result.groups[gi].fields[fi].value != valueBefore {
					t.Errorf("left should not change disabled field %q value", field.key)
				}

				// 3. Render contains "Coming soon"
				rendered := m.renderFieldToString(field, false)
				if !strings.Contains(rendered, "Coming soon") {
					t.Errorf("disabled field %q should render 'Coming soon', got: %s", field.key, rendered)
				}
			})
		}
	}
}

func hasEnabledFieldAfter(group WidgetGroup, index int) bool {
	for i := index + 1; i < len(group.fields); i++ {
		if !group.fields[i].disabled {
			return true
		}
	}
	return false
}

func hasEnabledFieldBefore(group WidgetGroup, index int) bool {
	for i := index - 1; i >= 0; i-- {
		if !group.fields[i].disabled {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Disabled field navigation — edge cases
// ---------------------------------------------------------------------------

func TestProfileWidget_EnterEditing_LandsOnFirstEnabled(t *testing.T) {
	// The Overlay group has 7 enabled fields and 1 disabled (ToggleKey).
	// Entering editing should land on the first enabled field.
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	overlayGroup := -1
	for i, group := range m.groups {
		if group.title == "Overlay settings" {
			overlayGroup = i
			break
		}
	}
	if overlayGroup == -1 {
		t.Fatal("could not find Overlay settings group")
	}

	m.focusedGroup = overlayGroup
	m.editing = false

	result, _ := m.Update(keyMsg("enter"))
	if !result.editing {
		t.Error("expected to enter editing mode")
	}

	// Should land on the first enabled field (index 0 = Enabled)
	field := result.groups[overlayGroup].fields[result.focusedField]
	if field.disabled {
		t.Error("expected to land on an enabled field")
	}

	// Navigate down to the last enabled field before ToggleKey,
	// then verify down doesn't move to ToggleKey (disabled)
	lastEnabled := -1
	for i := len(result.groups[overlayGroup].fields) - 1; i >= 0; i-- {
		if !result.groups[overlayGroup].fields[i].disabled {
			lastEnabled = i
			break
		}
	}

	result.focusedField = lastEnabled
	result, _ = result.Update(keyMsg("down"))
	if result.focusedField != lastEnabled {
		t.Errorf("expected down to stay on last enabled field %d, moved to %d", lastEnabled, result.focusedField)
	}
}

func TestProfileWidget_Navigation_FirstFieldDisabled(t *testing.T) {
	// CPU group: first field (Governor) is enabled, second (SMT) is
	// enabled, third (Affinity) is disabled. Test navigating down from
	// SMT skips Affinity (since it's last and disabled).
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	cpuGroup := -1
	for i, group := range m.groups {
		if group.title == "CPU settings" {
			cpuGroup = i
			break
		}
	}
	if cpuGroup == -1 {
		t.Fatal("could not find CPU settings group")
	}

	m.focusedGroup = cpuGroup
	m.editing = true

	// Find the last enabled field
	lastEnabled := -1
	for i := len(m.groups[cpuGroup].fields) - 1; i >= 0; i-- {
		if !m.groups[cpuGroup].fields[i].disabled {
			lastEnabled = i
			break
		}
	}
	if lastEnabled == -1 {
		t.Fatal("no enabled fields in CPU group")
	}

	// Position on last enabled field and try to go down
	m.focusedField = lastEnabled
	result, _ := m.Update(keyMsg("down"))
	// Should NOT move to the disabled field after it
	if result.focusedField != lastEnabled {
		t.Errorf("expected to stay on field %d (last enabled), moved to %d", lastEnabled, result.focusedField)
	}
}

// ---------------------------------------------------------------------------
// Editing — field value cycling
// ---------------------------------------------------------------------------

func TestProfileWidget_CycleValue(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	// Find a non-disabled field with options (e.g., shader_cache)
	var targetGroup, targetField int
	found := false
	for gi, group := range m.groups {
		for fi, field := range group.fields {
			if !field.disabled && len(field.options) > 1 && !field.usesModal {
				targetGroup = gi
				targetField = fi
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("could not find a non-disabled field with options")
	}

	m.focusedGroup = targetGroup
	m.focusedField = targetField
	m.editing = true

	valueBefore := m.groups[targetGroup].fields[targetField].value

	// Cycle right
	result, _ := m.Update(keyMsg("right"))
	valueAfter := result.groups[targetGroup].fields[targetField].value
	if valueAfter == valueBefore {
		t.Error("expected right to cycle field value")
	}
	if !result.modified {
		t.Error("expected modified flag to be set after cycling")
	}
}

func TestProfileWidget_CycleValue_DisabledRejected(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	// Find a disabled field
	var targetGroup, targetField int
	found := false
	for gi, group := range m.groups {
		for fi, field := range group.fields {
			if field.disabled {
				targetGroup = gi
				targetField = fi
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("could not find a disabled field")
	}

	m.focusedGroup = targetGroup
	m.focusedField = targetField
	m.editing = true

	valueBefore := m.groups[targetGroup].fields[targetField].value

	result, _ := m.Update(keyMsg("right"))
	if result.groups[targetGroup].fields[targetField].value != valueBefore {
		t.Error("expected right to be rejected on disabled field")
	}
}

// ---------------------------------------------------------------------------
// Save flow
// ---------------------------------------------------------------------------

func TestProfileWidget_Save(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)
	m.editing = true

	_, cmd := m.Update(keyMsg("s"))
	if cmd == nil {
		t.Error("expected save command from s key")
	}
	msg := execCmd(cmd)
	if _, ok := msg.(profileSaveMsg); !ok {
		t.Errorf("expected profileSaveMsg, got %T", msg)
	}
}

func TestProfileWidget_EscExitsEditing(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)
	m.editing = true

	result, _ := m.Update(keyMsg("esc"))
	if result.editing {
		t.Error("expected esc to exit editing mode")
	}
}

// ---------------------------------------------------------------------------
// VKD3D heap — field presence, toggle, and inline notice rendering
// ---------------------------------------------------------------------------

// findField scans the widget for the first field whose key matches and
// returns (groupIndex, fieldIndex, true). (0,0,false) when absent.
func findField(m ProfileWidgetModel, key string) (int, int, bool) {
	for gi, group := range m.groups {
		for fi, field := range group.fields {
			if field.key == key {
				return gi, fi, true
			}
		}
	}
	return 0, 0, false
}

// TestProfileWidget_VKD3DHeap_FieldPresent verifies that toggling the
// VKD3D Heap field in the Proton group flips the in-memory profile state.
// Covers acceptance: "TUI field is focused WHEN the user toggles it THEN
// the in-memory profile state reflects the change".
func TestProfileWidget_VKD3DHeap_FieldPresent(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	gi, fi, ok := findField(m, "vkd3d_heap")
	if !ok {
		t.Fatal("expected VKD3D heap field in profile widget")
	}
	if m.groups[gi].title != "Proton settings" {
		t.Errorf("expected vkd3d_heap in Proton settings group, got %q", m.groups[gi].title)
	}
	if m.groups[gi].fields[fi].label != "VKD3D heap" {
		t.Errorf("unexpected label: %q", m.groups[gi].fields[fi].label)
	}

	// Initial value: profile has VKD3DHeap=false, rendered as "(default)".
	if p.Proton.VKD3DHeap {
		t.Fatal("precondition: expected VKD3DHeap=false")
	}

	m.focusedGroup = gi
	m.focusedField = fi
	m.editing = true

	// Cycle right from "(default)" through options until the profile toggles.
	// options: ["(default)", "true", "false"] — one "right" lands on "true".
	result, _ := m.Update(keyMsg("right"))
	if !result.profile.Proton.VKD3DHeap {
		t.Errorf("expected VKD3DHeap=true after right-cycle, got false (value=%q)",
			result.groups[gi].fields[fi].value)
	}
	if !result.modified {
		t.Error("expected modified flag after toggling vkd3d_heap")
	}
}

// TestProfileWidget_VKD3DHeap_NoticeRendered verifies that when the field
// is enabled AND the injected notice source reports incompatibility, the
// widget renders the notice inline under the field.
// Covers acceptance: "TUI field is enabled AND resolver reports
// incompatibility WHEN widget renders THEN notice is visible inline".
func TestProfileWidget_VKD3DHeap_NoticeRendered(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)
	m.SetSize(120, 40)

	const notice = "⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 570.86.0)"
	m.SetVKD3DNoticeSource(func() string { return notice })

	// Render the full view and check for the notice. We don't assert on
	// styling — just presence.
	out := m.View()
	if !strings.Contains(out, "VKD3D heap") {
		t.Fatalf("expected VKD3D heap field in rendered output:\n%s", out)
	}
	if !strings.Contains(out, "580.94.16") {
		t.Errorf("expected driver version in rendered notice:\n%s", out)
	}
}

// TestProfileWidget_VKD3DHeap_NoticeSkippedWhenDisabled verifies that
// the notice source is not called when the toggle is false, and the
// notice text does not appear in the rendered output.
func TestProfileWidget_VKD3DHeap_NoticeSkippedWhenDisabled(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: false}}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)
	m.SetSize(120, 40)

	called := false
	m.SetVKD3DNoticeSource(func() string {
		called = true
		return "⚠ should not appear"
	})

	out := m.View()
	if called {
		t.Error("notice source should not be invoked when VKD3DHeap is false")
	}
	if strings.Contains(out, "should not appear") {
		t.Errorf("unexpected notice in output:\n%s", out)
	}
}

func TestProfileWidget_GridNavigation(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)
	m.editing = false
	m.focusedGroup = 0

	// Enter editing
	result, _ := m.Update(keyMsg("enter"))
	if !result.editing {
		t.Error("expected enter to start editing")
	}

	// The focused field should be the first non-disabled field
	group := result.groups[result.focusedGroup]
	for i := 0; i < result.focusedField; i++ {
		if !group.fields[i].disabled {
			t.Errorf("expected focusedField to be the first non-disabled field, but field %d is enabled", i)
			break
		}
	}
}
