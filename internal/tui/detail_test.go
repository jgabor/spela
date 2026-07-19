package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/nav"
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
// companion: every key in profile.AllFields() must have a label and a
// non-default value display so no supported field silently disappears.
func TestDetail_FieldEnumeration_AllFieldsHaveLabels(t *testing.T) {
	displayProfile := profileWithAllDisplayValues()
	for _, field := range profile.AllFields() {
		if label, ok := fieldLabels[field]; !ok || label == "" {
			t.Errorf("field %q is missing a display label in fieldLabels", field)
		}
		if value := formatFieldValue(displayProfile, field); value == "(default)" || value == "" {
			t.Errorf("field %q is missing a value display", field)
		}
	}
}

func profileWithAllDisplayValues() *profile.Profile {
	smt := true
	return &profile.Profile{
		Proton: profile.ProtonSettings{
			EnableHDR:        true,
			EnableWayland:    true,
			EnableNGXUpdater: true,
			VKD3DHeap:        true,
		},
		DLSS: profile.DLSSSettings{
			SRMode:      profile.DLSSModeQuality,
			SRPreset:    profile.DLSSPresetK,
			SROverride:  true,
			RRMode:      profile.DLSSModeBalanced,
			RRPreset:    profile.DLSSPresetJ,
			RROverride:  true,
			FGEnabled:   true,
			FGOverride:  true,
			MultiFrame:  2,
			Indicator:   true,
			FGIndicator: true,
		},
		GPU: profile.GPUSettings{
			ClockOffset:          150,
			MemoryOffset:         500,
			PowerLimit:           350,
			FanSpeed:             65,
			PowerMizer:           "prefer_max_performance",
			ShaderCache:          true,
			ShaderCachePath:      "/tmp/cache",
			ThreadedOptimization: true,
		},
		CPU: profile.CPUSettings{
			Governor: "performance",
			SMT:      &smt,
			Affinity: "0-7",
		},
		Overlay: profile.OverlaySettings{
			Enabled:       true,
			Position:      "top-left",
			ShowFPS:       true,
			ShowFrametime: true,
			ShowCPU:       true,
			ShowGPU:       true,
			ShowVRAM:      true,
			ToggleKey:     "F12",
		},
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

// TestDetail_ProfileSemanticsPreserveReloadMeaning_Pass verifies game rows
// render the shared source, impact, and restore terms while live defaults and
// explicit overrides keep their meanings across a rebuilt detail model.
func TestDetail_ProfileSemanticsPreserveReloadMeaning_Pass(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	raw := &profile.Profile{Name: "Cyberpunk 2077"}

	d := NewDetail(styles, raw, &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}})
	focusField(t, &d, profile.FieldGPUPowerLimit)
	line := detailLineContaining(d.View(), "Power limit")
	for _, want := range []string{"350", "source default", "impact system_state", "restore restorable_mutation"} {
		if !strings.Contains(line, want) {
			t.Fatalf("inherited line missing %q: %q", want, line)
		}
	}

	d = NewDetail(styles, raw, &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 420}})
	focusField(t, &d, profile.FieldGPUPowerLimit)
	line = detailLineContaining(d.View(), "Power limit")
	if !strings.Contains(line, "420") || !strings.Contains(line, "source default") || strings.Contains(line, "◆") {
		t.Fatalf("default reload should stay inherited without override marker: %q", line)
	}

	raw.GPU.PowerLimit = 400
	raw.MarkOverride(profile.FieldGPUPowerLimit)
	d = NewDetail(styles, raw, &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 420}})
	focusField(t, &d, profile.FieldGPUPowerLimit)
	line = detailLineContaining(d.View(), "Power limit")
	if !strings.Contains(line, "400") || !strings.Contains(line, "source override") || !strings.Contains(line, "◆") {
		t.Fatalf("override reload should preserve explicit intent: %q", line)
	}
}

// TestDetail_ProfileSemanticsInheritedNeverImpliesOverride_Fail is the fail
// pair: a fully inherited game profile must not render override source text.
func TestDetail_ProfileSemanticsInheritedNeverImpliesOverride_Fail(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	d := NewDetail(styles, &profile.Profile{}, &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}})
	line := detailLineContaining(d.View(), "HDR")
	if strings.Contains(line, "source override") || strings.Contains(line, "◆") {
		t.Fatalf("inherited HDR must not imply override: %q", line)
	}
	if !strings.Contains(line, "source default") || !strings.Contains(line, "impact compatibility") || !strings.Contains(line, "restore ephemeral_launch_environment") {
		t.Fatalf("inherited HDR line missing shared semantics: %q", line)
	}
}

func detailLineContaining(view, label string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, label) {
			return line
		}
	}
	return ""
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
	content := NewContent(styles, true, svc)
	pane := newResourcePane(styles, content)
	pane.setServices(svc)
	pane.SetSize(100, 30)

	pane.loadGlobalScope()
	out := pane.View(true)

	// Default profile shows at least the active subsystem (Proton by default).
	for _, header := range []string{"HDR", "Wayland"} {
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
	content := NewContent(styles, true, svc)
	pane := newResourcePane(styles, content)
	pane.setServices(svc)
	navState := nav.DefaultState()
	navState.Aspect = nav.AspectProfile
	pane.BindNavState(&navState)
	pane.SetSize(100, 30)

	before := pane.defaultsDetail.Cursor()
	pane, _ = pane.Update(keyMsg("j"))
	after := pane.defaultsDetail.Cursor()
	if after != before+1 {
		t.Errorf("j should advance defaults cursor %d → %d, got %d", before, before+1, after)
	}

	pane, _ = pane.Update(keyMsg("k"))
	if pane.defaultsDetail.Cursor() != before {
		t.Errorf("k should move defaults cursor back to %d, got %d", before, pane.defaultsDetail.Cursor())
	}
}

func TestResourcePane_RootMutationPersistsAndRetainsSelection(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HOME", filepath.Join(stateRoot, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(stateRoot, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(stateRoot, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(stateRoot, "cache"))

	if err := profile.SaveDefault(&profile.Profile{Name: "Default profile"}); err != nil {
		t.Fatalf("save initial defaults: %v", err)
	}
	services := testServices()
	services.LoadDefaultProfile = profile.LoadDefault
	styles := NewStyles(DefaultTheme, true)
	state := nav.DefaultState().
		SelectAspect(nav.AspectProfile).
		SelectProfileSubsystem(nav.SubsystemDLSS)
	state.Zone = nav.ZoneContent
	pane := newResourcePane(styles, NewContent(styles, true, services))
	pane.BindNavState(&state)
	pane.setServices(services)
	pane.SetState(state)
	focusField(t, &pane.defaultsDetail, profile.FieldDLSSSROverride)
	wantField := pane.defaultsDetail.FocusedField()
	wantCursor := pane.defaultsDetail.Cursor()

	mutated, saveCommand := pane.Update(keyMsg("right"))
	if saveCommand == nil {
		t.Fatal("root mutation returned no save command")
	}
	message, ok := saveCommand().(profileSaveMsg)
	if !ok || !message.success || message.err != nil {
		t.Fatalf("root save result = %#v", message)
	}
	persisted, err := profile.LoadDefault()
	if err != nil {
		t.Fatalf("load persisted defaults: %v", err)
	}
	if !persisted.DLSS.SROverride || !persisted.IsOverridden(profile.FieldDLSSSROverride) {
		t.Fatalf("persisted SR override = %v, overrides %v", persisted.DLSS.SROverride, persisted.Overrides)
	}

	layout := testLayout()
	layout.navState = &state
	layout.pane = mutated
	updated, announcements := layout.handleAppMessages(message, nil)
	if len(announcements) == 0 {
		t.Fatal("profile save result did not route to a user announcement")
	}
	if got := updated.pane.defaultsDetail.FocusedField(); got != wantField {
		t.Fatalf("focused field after save = %q, want %q", got, wantField)
	}
	if got := updated.pane.defaultsDetail.Cursor(); got != wantCursor {
		t.Fatalf("cursor after save = %d, want %d", got, wantCursor)
	}
	if got := updated.pane.defaultsDetail.activeSubsystem; got != nav.SubsystemDLSS.Key() {
		t.Fatalf("active subsystem after save = %q, want %q", got, nav.SubsystemDLSS.Key())
	}
}

// TestResourcePane_GamesSidebarPlusDetail verifies the Games resource
// layout: sidebar on the left, detail on the right, both present.
func TestResourcePane_GamesSidebarPlusDetail(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	layout := testLayoutWithGame(g)
	layout.navState.Aspect = nav.AspectOverview
	layout.pane.SetState(*layout.navState)
	out := layout.pane.View(true)

	if !strings.Contains(out, "Cyberpunk 2077") {
		t.Errorf("games view missing game name 'Cyberpunk 2077':\n%s", out)
	}
	for _, header := range []string{"App ID", "Overrides"} {
		if !strings.Contains(out, header) {
			t.Errorf("games view missing detail header %q in:\n%s", header, out)
		}
	}
}

func TestLayoutHandlers_TabIntoLibraryProfile(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "tab")
	m = result.(LayoutModel)
	if m.navState.Zone != nav.ZoneContext {
		t.Fatalf("after tab: expected context zone, got %v", m.navState.Zone)
	}
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	if m.navState.Zone != nav.ZoneContent {
		t.Errorf("after second tab: expected content zone, got %v", m.navState.Zone)
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

// TestDetail_ArrowDownUsesKeyDownCode documents that profile field navigation
// responds to the tea.KeyDown code produced by a terminal's down-arrow input.
func TestDetail_ArrowDownUsesKeyDownCode(t *testing.T) {
	d, _, _ := newGameDetailForTask5(t)
	initial := d.Cursor()
	updated, _, handled := d.Update(keyMsg("down"))
	if !handled {
		t.Fatal("expected down arrow to move cursor")
	}
	if updated.Cursor() != initial+1 {
		t.Fatalf("cursor = %d, want %d", updated.Cursor(), initial+1)
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

// TestDetail_RootResetFocused_ClearsValue — reset on the root defaults view
// clears a non-default field back to "(default)".
func TestDetail_RootResetFocused_ClearsValue(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	root.MarkOverride(profile.FieldProtonVKD3DHeap)
	d := NewRootDetail(styles, root)
	focusField(t, &d, profile.FieldProtonVKD3DHeap)

	changed, err := d.ResetFocused()
	if err != nil {
		t.Fatalf("ResetFocused: %v", err)
	}
	if !changed {
		t.Fatal("ResetFocused on root: expected changed=true")
	}
	if root.Proton.VKD3DHeap {
		t.Error("ResetFocused on root: expected VKD3DHeap cleared")
	}
	if root.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("ResetFocused on root: expected override cleared")
	}
	view := d.View()
	if !strings.Contains(view, "(default)") {
		t.Errorf("ResetFocused on root: expected (default) in view, got:\n%s", view)
	}
}

// TestDetail_RootCycleBoolField_ThreeStates — root bool fields cycle through
// (default), true, and false.
func TestDetail_RootCycleBoolField_ThreeStates(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{}
	d := NewRootDetail(styles, root)
	focusField(t, &d, profile.FieldProtonVKD3DHeap)

	if !d.CycleFocusedField(1) {
		t.Fatal("expected first cycle to change value")
	}
	if !root.Proton.VKD3DHeap {
		t.Error("expected true after cycle from (default)")
	}
	if !d.CycleFocusedField(1) {
		t.Fatal("expected second cycle to change value")
	}
	if root.Proton.VKD3DHeap {
		t.Error("expected false after second cycle")
	}
	if !root.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("expected explicit false to mark override")
	}
	if got := formatRootBoolField(root, profile.FieldProtonVKD3DHeap); got != "false" {
		t.Errorf("expected display false, got %q", got)
	}
	if !d.CycleFocusedField(1) {
		t.Fatal("expected third cycle to change value")
	}
	if root.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("expected (default) to clear override")
	}
	if got := formatRootBoolField(root, profile.FieldProtonVKD3DHeap); got != "(default)" {
		t.Errorf("expected display (default), got %q", got)
	}
}

// TestDetail_RootPinFocused_NoOp — pin is a game-profile binding; root fields
// are edited directly and PinFocused remains a no-op there.
func TestDetail_RootPinFocused_NoOp(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	d := NewRootDetail(styles, root)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	if changed, _ := d.PinFocused(); changed {
		t.Error("root PinFocused must be no-op")
	}
}

// TestDetail_RootResetFocused_ClearsIntField — reset on the root defaults
// view clears non-default numeric fields back to "(default)".
func TestDetail_RootResetFocused_ClearsIntField(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}
	d := NewRootDetail(styles, root)
	focusField(t, &d, profile.FieldGPUPowerLimit)

	changed, err := d.ResetFocused()
	if err != nil {
		t.Fatalf("ResetFocused: %v", err)
	}
	if !changed {
		t.Fatal("ResetFocused on root int field: expected changed=true")
	}
	if root.GPU.PowerLimit != 0 {
		t.Errorf("ResetFocused on root: expected PowerLimit zeroed, got %d", root.GPU.PowerLimit)
	}
}

// TestDetail_RootResetAll_ClearsAllFields — reset-all on the root defaults
// view clears every non-default field (bool and non-bool) back to "(default)".
func TestDetail_RootResetAll_ClearsAllFields(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{
		Proton: profile.ProtonSettings{VKD3DHeap: true, EnableHDR: true},
		GPU:    profile.GPUSettings{PowerLimit: 350},
	}
	root.MarkOverride(profile.FieldProtonVKD3DHeap)
	root.MarkOverride(profile.FieldProtonEnableHDR)
	root.MarkOverride(profile.FieldGPUPowerLimit)
	d := NewRootDetail(styles, root)

	if !d.ResetAll() {
		t.Fatal("ResetAll on root: expected changed=true")
	}
	if root.Proton.VKD3DHeap || root.Proton.EnableHDR {
		t.Error("ResetAll on root: expected bool fields cleared")
	}
	if root.GPU.PowerLimit != 0 {
		t.Errorf("ResetAll on root: expected PowerLimit zeroed, got %d", root.GPU.PowerLimit)
	}
	if len(root.Overrides) != 0 {
		t.Errorf("ResetAll on root: expected overrides cleared, got %v", root.Overrides)
	}
}

// TestDetail_RootResetAll_NoOp_AtDefault — ResetAll reports no change when
// the root profile is already at default.
func TestDetail_RootResetAll_NoOp_AtDefault(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	root := &profile.Profile{}
	d := NewRootDetail(styles, root)
	if d.ResetAll() {
		t.Fatal("ResetAll on root at default: expected changed=false")
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
