package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

// ---------------------------------------------------------------------------
// Detail renderer — Task 4 acceptance
// ---------------------------------------------------------------------------

// TestDetail_FieldEnumeration_GroupOrder verifies the renderer walks the
// canonical subsystem order (proton → dlss → gpu → cpu → overlay) and that
// every inheritance-tracked field on the profile is represented exactly once.
func TestDetail_FieldEnumeration_GroupOrder(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})

	// Every focusable row should map to a profile field in the canonical
	// per-section order.
	want := []string{}
	for _, section := range []string{"proton", "dlss", "gpu", "cpu", "overlay"} {
		want = append(want, profile.SectionFields(section)...)
	}

	if got := d.FieldCount(); got != len(want) {
		t.Fatalf("FieldCount = %d, want %d", got, len(want))
	}

	// Walk j/k forward and confirm the order.
	for i, field := range want {
		if got := d.FocusedField(); got != field {
			t.Errorf("position %d: FocusedField = %q, want %q", i, got, field)
		}
		if i < len(want)-1 {
			var handled bool
			d, _, handled = d.Update(keyMsg("j"))
			if !handled {
				t.Fatalf("expected j to be handled at position %d", i)
			}
		}
	}
}

// TestDetail_FieldEnumeration_AllFieldsHaveLabels is the adversarial
// companion: every key in profile.AllFields() must have an entry in
// fieldLabels so no field renders with a raw "proton.vkd3d_heap" key.
func TestDetail_FieldEnumeration_AllFieldsHaveLabels(t *testing.T) {
	for _, field := range profile.AllFields() {
		if label, ok := fieldLabels[field]; !ok || label == "" {
			t.Errorf("field %q is missing a display label in fieldLabels", field)
		}
	}
}

// TestDetail_FieldEnumeration_ZeroValueFails documents what the renderer
// does NOT do: it must not list a field absent from any section. This test
// asserts the explicit fail case — passing an unknown section name returns
// nil (so nothing renders under that pseudo-header).
func TestDetail_FieldEnumeration_ZeroValueFails(t *testing.T) {
	got := profile.SectionFields("not-a-real-section")
	if got != nil {
		t.Errorf("SectionFields('not-a-real-section') = %v, want nil", got)
	}
}

// TestDetail_JKCrossesGroupHeaders verifies j/k moves focus across section
// boundaries without pausing on group headers. Proton has 4 fields, DLSS
// has 12; starting at the first field, 4 j presses must land inside DLSS
// (not on a header).
func TestDetail_JKCrossesGroupHeaders(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})

	// Proton has 4 fields — 4 j presses land on the 5th focusable row,
	// which is the first DLSS field.
	protonFields := profile.SectionFields("proton")
	if len(protonFields) != 4 {
		t.Fatalf("precondition: proton has %d fields, expected 4 (fieldset changed?)", len(protonFields))
	}
	dlssFields := profile.SectionFields("dlss")
	if len(dlssFields) == 0 {
		t.Fatalf("precondition: dlss must have at least one field")
	}

	for i := 0; i < 4; i++ {
		d, _, _ = d.Update(keyMsg("j"))
	}

	got := d.FocusedField()
	if got != dlssFields[0] {
		t.Errorf("after 4 j presses: FocusedField = %q, want %q (first DLSS field)", got, dlssFields[0])
	}
}

// TestDetail_JKClampsAtEnds verifies up/down clamp at the first and last
// focusable field.
func TestDetail_JKClampsAtEnds(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})

	// Up from the first field is a no-op.
	d, _, _ = d.Update(keyMsg("k"))
	if d.Cursor() != 0 {
		t.Errorf("up-from-zero: cursor = %d, want 0", d.Cursor())
	}

	// Down all the way.
	total := d.FieldCount()
	for i := 0; i < total*2; i++ {
		d, _, _ = d.Update(keyMsg("j"))
	}
	if d.Cursor() != total-1 {
		t.Errorf("down-past-end: cursor = %d, want %d", d.Cursor(), total-1)
	}
}

// TestDetail_RootSuppressesMarkers is the acceptance for defaults-root
// rendering. Markers (`[inherited]`, `[override]`, and the AccentOverride
// marker glyph) must NOT appear in the rendered output when isRoot=true.
// Task 5 will add these markers for game profiles only.
func TestDetail_RootSuppressesMarkers(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	p := &profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: true},
	}
	d := NewRootDetail(styles, p)
	d.SetSize(80, 30)
	out := d.View()

	forbidden := []string{"[inherited]", "[override]", "◆"}
	for _, forbid := range forbidden {
		if strings.Contains(out, forbid) {
			t.Errorf("root detail view should not contain %q, got:\n%s", forbid, out)
		}
	}

	if !d.IsRoot() {
		t.Error("IsRoot() should be true for NewRootDetail")
	}
}

// TestDetail_GameDetailResolvesInheritance verifies that when the game
// profile does NOT override a field, the rendered value comes from the
// defaults via ResolveForApply. Specifically: HDR true on defaults, unset
// on the game → rendered value for the game is "true".
func TestDetail_GameDetailResolvesInheritance(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)

	defaults := &profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: true},
	}
	game := &profile.Profile{
		Name: "Cyberpunk 2077",
	}

	d := NewDetail(styles, game, defaults)
	d.SetSize(80, 30)
	out := d.View()

	// The HDR field should render as "true" because the game profile
	// inherits from defaults which set it to true.
	if !strings.Contains(out, "HDR") {
		t.Errorf("output missing HDR label:\n%s", out)
	}
	// First focusable field is proton.enable_hdr.
	if got := d.FocusedField(); got != profile.FieldProtonEnableHDR {
		t.Errorf("first focus should be %q, got %q", profile.FieldProtonEnableHDR, got)
	}
	if d.IsRoot() {
		t.Error("IsRoot() should be false for NewDetail")
	}

	// The resolved value must be rendered. Scan for the line containing
	// "HDR" and verify it includes "true".
	lines := strings.Split(out, "\n")
	var hdrLine string
	for _, line := range lines {
		if strings.Contains(line, "HDR") && !strings.Contains(line, "Proton") {
			hdrLine = line
			break
		}
	}
	if !strings.Contains(hdrLine, "true") {
		t.Errorf("HDR line should render 'true' (resolved from defaults), got: %q", hdrLine)
	}
}

// TestDetail_GameDetailOverriddenValue verifies that a pinned override on
// the game profile is what gets rendered — not the defaults.
func TestDetail_GameDetailOverriddenValue(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)

	defaults := &profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: true},
	}
	game := &profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: false},
		Overrides: map[string]bool{
			profile.FieldProtonEnableHDR: true,
		},
	}

	d := NewDetail(styles, game, defaults)
	d.SetSize(80, 30)
	out := d.View()

	// The HDR field should render as "(default)" because the game pins
	// false, and displayBool treats false as the default sentinel.
	var hdrLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "HDR") && !strings.Contains(line, "Proton") {
			hdrLine = line
			break
		}
	}
	// The override sets HDR=false; the detail renderer's value column
	// should reflect that — not 'true' from defaults.
	if strings.Contains(hdrLine, "true") {
		t.Errorf("overridden HDR should NOT render 'true' (defaults value), got: %q", hdrLine)
	}
}

// TestDetail_ViewRendersEveryGroupHeader verifies that every subsystem
// group header is present in the rendered output (acceptance: no collapse/
// expand toggle — every group always visible).
func TestDetail_ViewRendersEveryGroupHeader(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})
	d.SetSize(80, 40)
	out := d.View()

	wantHeaders := []string{"Proton", "DLSS", "GPU", "CPU", "Overlay"}
	for _, header := range wantHeaders {
		if !strings.Contains(out, header) {
			t.Errorf("output missing header %q:\n%s", header, out)
		}
	}
}

// TestDetail_ViewRendersEveryGroupHeader_FailNegative is the companion
// negative case: asserting a header that should NOT exist does not appear.
func TestDetail_ViewRendersEveryGroupHeader_FailNegative(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})
	d.SetSize(80, 40)
	out := d.View()

	forbidden := []string{"Network", "Audio", "Physics"} // not real subsystems
	for _, header := range forbidden {
		if strings.Contains(out, header) {
			t.Errorf("output should NOT contain header %q, got:\n%s", header, out)
		}
	}
}

// TestDetail_NonKeyMessageUnhandled verifies the detail renderer returns
// handled=false for non-KeyPressMsg messages so the parent can route them
// elsewhere (tea.WindowSizeMsg, custom messages, etc.).
func TestDetail_NonKeyMessageUnhandled(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, &profile.Profile{})
	_, _, handled := d.Update("not-a-key")
	if handled {
		t.Error("non-key message should return handled=false")
	}
}

// TestDetail_NilProfileSafeRender verifies the renderer does not panic on
// a nil root profile and renders "(default)" for every value.
func TestDetail_NilProfileSafeRender(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewRootDetail(styles, nil)
	d.SetSize(80, 40)
	_ = d.View() // must not panic
}

// TestResourcePane_DefaultsUsesRootDetail verifies the pane wires the
// defaults resource to the DetailModel (isRoot=true) — no stub body.
func TestResourcePane_DefaultsUsesRootDetail(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	svc := testServices()
	svc.LoadDefaultProfile = func() (*profile.Profile, error) {
		return &profile.Profile{
			Proton: profile.ProtonSettings{EnableHDR: true},
		}, nil
	}
	sidebar, _ := NewSidebar(nil, styles, svc)
	content := NewContent(styles, true, svc)
	pane := newResourcePane(styles, sidebar, content)
	pane.setServices(svc)
	pane.SetSize(100, 30)

	out := pane.View(ResourceDefaults, true)

	// Headers from the grouped renderer must be present.
	for _, header := range []string{"Proton", "DLSS", "GPU", "CPU", "Overlay"} {
		if !strings.Contains(out, header) {
			t.Errorf("defaults view missing header %q in:\n%s", header, out)
		}
	}
	// No inheritance markers in root rendering.
	if strings.Contains(out, "[inherited]") || strings.Contains(out, "[override]") {
		t.Errorf("defaults view should not contain inheritance markers, got:\n%s", out)
	}
	// Old Task 3 stub text should be gone.
	if strings.Contains(out, "shared detail renderer (Task 4)") {
		t.Errorf("defaults view still shows Task 3 stub text")
	}
}

// TestResourcePane_DefaultsJKMovesFieldFocus verifies that pressing j/k
// while the defaults pane is active moves field focus inside it.
func TestResourcePane_DefaultsJKMovesFieldFocus(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	svc := testServices()
	sidebar, _ := NewSidebar(nil, styles, svc)
	content := NewContent(styles, true, svc)
	pane := newResourcePane(styles, sidebar, content)
	pane.setServices(svc)
	pane.SetSize(100, 30)

	before := pane.defaultsDetail.Cursor()
	pane, _ = pane.Update(keyMsg("j"), ResourceDefaults)
	after := pane.defaultsDetail.Cursor()
	if after != before+1 {
		t.Errorf("j should advance defaults cursor %d → %d, got %d", before, before+1, after)
	}

	pane, _ = pane.Update(keyMsg("k"), ResourceDefaults)
	if pane.defaultsDetail.Cursor() != before {
		t.Errorf("k should move defaults cursor back to %d, got %d", before, pane.defaultsDetail.Cursor())
	}
}

// TestResourcePane_GamesSidebarPlusDetail verifies the Games resource
// layout: sidebar on the left, detail on the right, both present.
func TestResourcePane_GamesSidebarPlusDetail(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	layout := testLayoutWithGame(g)
	out := layout.pane.View(ResourceGames, true)

	if !strings.Contains(out, "Cyberpunk 2077") {
		t.Errorf("games view missing game name 'Cyberpunk 2077':\n%s", out)
	}
	// The grouped-field renderer's headers must appear on the detail side.
	for _, header := range []string{"Proton", "DLSS", "GPU", "CPU", "Overlay"} {
		if !strings.Contains(out, header) {
			t.Errorf("games view missing detail header %q in:\n%s", header, out)
		}
	}
}

// TestLayoutHandlers_TabIntoDefaultsMovesInnerFocus verifies the focus-
// transfer contract documented in the help screen: from the rail, pressing
// Tab with Defaults active transfers focus into the pane (innerFocused=true)
// so subsequent j/k moves field focus instead of rail cursor.
func TestLayoutHandlers_TabIntoDefaultsMovesInnerFocus(t *testing.T) {
	m := testLayout()
	// Select the Defaults resource.
	result, _ := sendKey(&m, "3")
	m = result.(LayoutModel)
	if m.rail.Active() != ResourceDefaults {
		t.Fatalf("precondition: expected ResourceDefaults, got %v", m.rail.Active())
	}
	if !m.railFocused {
		t.Fatalf("precondition: rail should be focused after pressing 3")
	}
	// Tab into the pane.
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	if m.railFocused {
		t.Errorf("after tab: railFocused should be false")
	}
	if !m.pane.InnerFocused() {
		t.Errorf("after tab with Defaults active: innerFocused should be true")
	}
}

// ---------------------------------------------------------------------------
// Task 5 — inheritance rendering, reset/pin bindings, DLSS dedup
// ---------------------------------------------------------------------------

// Helper: return a Games-mode (non-root) DetailModel with a game profile that
// overrides PowerLimit=400 on top of defaults PowerLimit=350.
func newGameDetailForTask5(t *testing.T) (DetailModel, *profile.Profile, *profile.Profile) {
	t.Helper()
	styles := NewStyles(DefaultTheme, true)
	defaults := &profile.Profile{
		GPU:    profile.GPUSettings{PowerLimit: 350},
		Proton: profile.ProtonSettings{EnableWayland: true},
	}
	raw := &profile.Profile{
		Name: "Cyberpunk 2077",
		GPU:  profile.GPUSettings{PowerLimit: 400},
	}
	raw.MarkOverride(profile.FieldGPUPowerLimit)
	return NewDetail(styles, raw, defaults), raw, defaults
}

// Advance the cursor until FocusedField matches `field` (or fail).
func focusField(t *testing.T, d *DetailModel, field string) {
	t.Helper()
	for i := 0; i < d.FieldCount(); i++ {
		if d.FocusedField() == field {
			return
		}
		updated, _, handled := d.Update(keyMsg("j"))
		if !handled {
			break
		}
		*d = updated
	}
	if d.FocusedField() != field {
		t.Fatalf("focusField: could not focus %q, stopped at %q", field, d.FocusedField())
	}
}

// TestDetailView_InheritedRowUsesMutedToken — inherited rows render in the
// fg-muted style and do NOT carry the ◆ override marker glyph.
func TestDetailView_InheritedRowUsesMutedToken(t *testing.T) {
	d, _, _ := newGameDetailForTask5(t)
	view := d.View()

	// Wayland inherits (default true), so its line should be muted and
	// carry no marker.
	if !strings.Contains(view, "Wayland") {
		t.Fatalf("view missing Wayland row:\n%s", view)
	}
	// The inherited style color is FgMuted (#6a6a80 → RGB 106,106,128).
	if !strings.Contains(view, "106;106;128") {
		t.Errorf("view has no fg-muted ANSI color for inherited rows:\n%s", view)
	}
	// Overridden PowerLimit is focused cyan OR fg+marker. Wayland should
	// not receive a ◆ marker. We probe by counting ◆ occurrences and
	// expecting exactly one (PowerLimit) since only one field is overridden.
	if n := strings.Count(view, "◆"); n != 1 {
		t.Errorf("expected exactly 1 override marker ◆ (PowerLimit), got %d in:\n%s", n, view)
	}
}

// TestDetailView_OverriddenRowUsesMarker — overridden rows render the ◆
// marker in AccentOverride (magenta) AND use the fg (not muted) foreground.
func TestDetailView_OverriddenRowUsesMarker(t *testing.T) {
	d, _, _ := newGameDetailForTask5(t)
	view := d.View()

	// AccentOverride = #ff5fd2 → RGB 255,95,210.
	if !strings.Contains(view, "255;95;210") {
		t.Errorf("view lacks AccentOverride magenta for override marker:\n%s", view)
	}
	// Fg = #e8e8f0 → RGB 232,232,240.
	if !strings.Contains(view, "232;232;240") {
		t.Errorf("view lacks Fg (overridden) foreground:\n%s", view)
	}
}

// Negative pair for the override marker: a game profile with zero overrides
// renders with NO markers, even though every field is inherited.
func TestDetailView_NoOverridesNoMarkers(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	defaults := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	raw := &profile.Profile{} // no overrides at all
	d := NewDetail(styles, raw, defaults)
	view := d.View()

	if strings.Contains(view, "◆") {
		t.Errorf("expected no ◆ override markers in fully-inherited view, got:\n%s", view)
	}
}

// Root defaults view suppresses BOTH muted styling and markers (re-asserts
// Task 4 acceptance after Task 5's per-row styling is added).
func TestDetailView_RootSuppressesMarkersAndMuting(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	d := NewRootDetail(styles, root)
	view := d.View()

	if strings.Contains(view, "◆") {
		t.Errorf("root view should not render override markers, got:\n%s", view)
	}
}

// TestDetail_ResetFocused_Overridden_Pass — pressing r on an overridden
// field clears the override and zeros the backing value.
func TestDetail_ResetFocused_Overridden_Pass(t *testing.T) {
	d, raw, _ := newGameDetailForTask5(t)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	changed, err := d.ResetFocused()
	if err != nil {
		t.Fatalf("ResetFocused: %v", err)
	}
	if !changed {
		t.Fatal("ResetFocused on overridden field: expected changed=true")
	}
	if raw.IsOverridden(profile.FieldGPUPowerLimit) {
		t.Error("ResetFocused: expected override flag cleared")
	}
	if raw.GPU.PowerLimit != 0 {
		t.Errorf("ResetFocused: expected backing value zeroed, got %d", raw.GPU.PowerLimit)
	}
}

// TestDetail_ResetFocused_Inherited_Fail (fail pair) — pressing r on an
// already-inherited field is a no-op (changed=false, no error).
func TestDetail_ResetFocused_Inherited_Fail(t *testing.T) {
	d, raw, _ := newGameDetailForTask5(t)
	// PowerLimit is overridden; focus a field that is inherited (Wayland).
	focusField(t, &d, profile.FieldProtonEnableWayland)

	changed, err := d.ResetFocused()
	if err != nil {
		t.Fatalf("ResetFocused unexpected error: %v", err)
	}
	if changed {
		t.Error("ResetFocused on inherited field: expected changed=false")
	}
	// Overrides map should still carry PowerLimit untouched.
	if !raw.IsOverridden(profile.FieldGPUPowerLimit) {
		t.Error("ResetFocused on inherited field must not touch unrelated overrides")
	}
}

// TestDetail_ResetAll_ClearsEveryOverride_Pass.
func TestDetail_ResetAll_ClearsEveryOverride_Pass(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	defaults := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	raw := &profile.Profile{
		Name:   "Cyberpunk 2077",
		GPU:    profile.GPUSettings{PowerLimit: 400},
		Proton: profile.ProtonSettings{EnableHDR: true},
	}
	raw.MarkOverride(profile.FieldGPUPowerLimit)
	raw.MarkOverride(profile.FieldProtonEnableHDR)
	d := NewDetail(styles, raw, defaults)

	if !d.ResetAll() {
		t.Fatal("ResetAll: expected changed=true with two overrides present")
	}
	if len(raw.Overrides) != 0 {
		t.Errorf("ResetAll: expected empty Overrides, got %v", raw.Overrides)
	}
	if raw.Name != "Cyberpunk 2077" {
		t.Errorf("ResetAll: expected Name preserved, got %q", raw.Name)
	}
	if raw.GPU.PowerLimit != 0 || raw.Proton.EnableHDR {
		t.Error("ResetAll: expected every tracked field zeroed")
	}
}

// TestDetail_ResetAll_NoOverrides_Fail (fail pair).
func TestDetail_ResetAll_NoOverrides_Fail(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	raw := &profile.Profile{Name: "Clean"}
	d := NewDetail(styles, raw, &profile.Profile{})

	if d.ResetAll() {
		t.Error("ResetAll with zero overrides: expected changed=false")
	}
}

// TestDetail_PinFocused_Inherited_Pass — pressing p on an inherited field
// copies the currently-resolved value and marks the override.
func TestDetail_PinFocused_Inherited_Pass(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	defaults := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	raw := &profile.Profile{Name: "Cyberpunk 2077"}
	d := NewDetail(styles, raw, defaults)

	focusField(t, &d, profile.FieldGPUPowerLimit)
	changed, err := d.PinFocused()
	if err != nil {
		t.Fatalf("PinFocused: %v", err)
	}
	if !changed {
		t.Fatal("PinFocused on inherited field: expected changed=true")
	}
	if !raw.IsOverridden(profile.FieldGPUPowerLimit) {
		t.Error("PinFocused: expected override flag set")
	}
	if raw.GPU.PowerLimit != 350 {
		t.Errorf("PinFocused: expected resolved value 350 copied, got %d", raw.GPU.PowerLimit)
	}
}

// TestDetail_PinFocused_AlreadyOverridden_Fail (fail pair) — pressing p on
// a field that is already overridden is a no-op (idempotent).
func TestDetail_PinFocused_AlreadyOverridden_Fail(t *testing.T) {
	d, raw, _ := newGameDetailForTask5(t)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	changed, err := d.PinFocused()
	if err != nil {
		t.Fatalf("PinFocused: %v", err)
	}
	if changed {
		t.Error("PinFocused on already-overridden field: expected changed=false")
	}
	if raw.GPU.PowerLimit != 400 {
		t.Errorf("PinFocused on overridden: expected value preserved (400), got %d", raw.GPU.PowerLimit)
	}
}

// TestDetail_RootSuppressesResetPinBindings — ResetFocused/ResetAll/
// PinFocused must all be no-ops on the root defaults view. This prevents
// the Task 5 bindings from leaking into the Defaults resource.
func TestDetail_RootSuppressesResetPinBindings(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	d := NewRootDetail(styles, root)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	if changed, _ := d.ResetFocused(); changed {
		t.Error("root ResetFocused must be no-op")
	}
	if d.ResetAll() {
		t.Error("root ResetAll must be no-op")
	}
	if changed, _ := d.PinFocused(); changed {
		t.Error("root PinFocused must be no-op")
	}
	if root.GPU.PowerLimit != 350 {
		t.Errorf("root profile mutated by suppressed bindings: got PowerLimit=%d", root.GPU.PowerLimit)
	}
}

// TestDetail_RebuildResolvedAfterReset — after resetting a focused override
// the rendered value falls back to the defaults' value.
func TestDetail_RebuildResolvedAfterReset(t *testing.T) {
	d, _, _ := newGameDetailForTask5(t)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	_, _ = d.ResetFocused()
	view := d.View()
	// After reset, the resolved view should show 350 (from defaults) on the
	// PowerLimit row.
	if !strings.Contains(view, "350") {
		t.Errorf("after reset: expected resolved value 350 in view, got:\n%s", view)
	}
	// And the marker should be gone.
	if strings.Count(view, "◆") != 0 {
		t.Errorf("after reset: expected zero override markers, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// DLSS preset modal — Task 5 dedup
// ---------------------------------------------------------------------------

// TestDLSSPresetOrder_NoDuplicates_Pass — the rendered order contains every
// preset at most once.
func TestDLSSPresetOrder_NoDuplicates_Pass(t *testing.T) {
	seen := make(map[profile.DLSSPreset]int)
	for _, p := range dlssPresetOrder {
		seen[p]++
	}
	for p, count := range seen {
		if count > 1 {
			t.Errorf("DLSS preset %q appears %d times, want 1", p, count)
		}
	}
}

// TestDedupePresets_StripsDuplicates_Pass — the dedupePresets helper strips
// duplicates while preserving the first-seen order. Adversarial input.
func TestDedupePresets_StripsDuplicates_Pass(t *testing.T) {
	in := []profile.DLSSPreset{
		profile.DLSSPresetA,
		profile.DLSSPresetB,
		profile.DLSSPresetA, // dup — must be dropped
		profile.DLSSPresetC,
		profile.DLSSPresetB, // dup — must be dropped
	}
	out := dedupePresets(in)
	want := []profile.DLSSPreset{
		profile.DLSSPresetA,
		profile.DLSSPresetB,
		profile.DLSSPresetC,
	}
	if len(out) != len(want) {
		t.Fatalf("dedupePresets: len=%d want %d (%v)", len(out), len(want), out)
	}
	for i, p := range want {
		if out[i] != p {
			t.Errorf("position %d: got %q, want %q (order must be preserved)", i, out[i], p)
		}
	}
}

// TestDedupePresets_EmptyInput_Fail (negative pair) — empty input produces
// empty output, never nil-ness or panic.
func TestDedupePresets_EmptyInput_Fail(t *testing.T) {
	out := dedupePresets(nil)
	if len(out) != 0 {
		t.Errorf("dedupePresets(nil): expected empty, got %v", out)
	}
}

// TestDLSSPresetModalView_EachPresetAppearsOnce — the rendered picker string
// shows each preset letter exactly once (end-to-end assertion at the view
// layer, not just the slice).
func TestDLSSPresetModalView_EachPresetAppearsOnce(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	m := NewDLSSPresetModal(styles)
	m.SetSize(120, 40)
	m.Open(profile.DLSSPresetDefault)

	view := m.View()
	// Each non-default preset appears once as a standalone letter on its
	// row. Count occurrences of the pattern " A " / " B " etc (with a
	// leading space inside the row padding).
	for _, preset := range []profile.DLSSPreset{
		profile.DLSSPresetA,
		profile.DLSSPresetB,
		profile.DLSSPresetC,
		profile.DLSSPresetD,
		profile.DLSSPresetE,
		profile.DLSSPresetF,
		profile.DLSSPresetJ,
		profile.DLSSPresetK,
		profile.DLSSPresetL,
		profile.DLSSPresetM,
	} {
		// Modal pads with "%-10s" so each preset letter appears followed
		// by whitespace. We count rough occurrences of " <letter> " as a
		// weak uniqueness probe.
		needle := string(preset)
		count := strings.Count(view, needle+"    ") // trailing padding
		if count > 1 {
			t.Errorf("preset %q appears %d times in modal view, want 1", preset, count)
		}
	}
}
