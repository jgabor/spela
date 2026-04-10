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

func TestProfileWidget_EnterEditing_SkipsDisabledFirst(t *testing.T) {
	// The Overlay group has ALL fields disabled. When entering editing
	// on that group, focusedField should land on 0 (the loop doesn't
	// find a non-disabled field, so it stays at 0).
	g := testGame("Cyberpunk 2077")
	p := &profile.Profile{}
	styles := NewStyles(DefaultTheme, true)
	m := NewProfileWidget(g, p, styles)

	// Find the Overlay group (all fields disabled)
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

	// Enter editing — all fields are disabled
	result, _ := m.Update(keyMsg("enter"))
	if !result.editing {
		t.Error("expected to enter editing mode")
	}

	// Navigation in all-disabled group should not move
	startField := result.focusedField
	result, _ = result.Update(keyMsg("down"))
	if result.focusedField != startField {
		t.Error("expected down to stay put in all-disabled group")
	}
	result, _ = result.Update(keyMsg("up"))
	if result.focusedField != startField {
		t.Error("expected up to stay put in all-disabled group")
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
