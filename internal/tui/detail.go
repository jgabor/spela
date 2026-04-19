// Package tui — detail renderer for the Games and Defaults resources.
//
// This file implements the shared single-column, grouped-by-subsystem profile
// detail renderer per .agentera/DECISIONS.md Decision 1. It is consumed in two
// contexts:
//
//   - Games resource: renders the currently selected game's resolved profile
//     (ResolveForApply) with isRoot=false. Task 5 will layer inheritance
//     markers on top of this renderer; Task 4 only wires the structural
//     spine (groups, fields, focus navigation).
//   - Defaults resource: renders the default profile as root with isRoot=true.
//     No inherited/overridden markers, no reset/pin keybindings.
//
// Task 4 scope deliberately excludes inheritance markers and r/shift+r/p
// keybindings — those land in Task 5.
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/profile"
)

// detailSectionOrder is the canonical rendering order for subsystem groups
// (per .agentera/DECISIONS.md Decision 1). Load-bearing: tests assert this.
var detailSectionOrder = []string{"proton", "dlss", "gpu", "cpu", "overlay"}

// detailSectionTitle maps a subsystem key to its human-readable header.
var detailSectionTitle = map[string]string{
	"proton":  "Proton",
	"dlss":    "DLSS",
	"gpu":     "GPU",
	"cpu":     "CPU",
	"overlay": "Overlay",
}

// fieldLabels maps each tracked field key to its display label. Kept close to
// the CLI labels so a user reading `spela proton show` sees the same text in
// the TUI. New fields added to profile.AllFields() need an entry here.
var fieldLabels = map[string]string{
	profile.FieldProtonEnableHDR:        "HDR",
	profile.FieldProtonEnableWayland:    "Wayland",
	profile.FieldProtonEnableNGXUpdater: "NGX updater",
	profile.FieldProtonVKD3DHeap:        "VKD3D heap",

	profile.FieldDLSSSRMode:        "SR mode",
	profile.FieldDLSSSRPreset:      "SR preset",
	profile.FieldDLSSSRModelPreset: "SR model preset",
	profile.FieldDLSSSROverride:    "SR override",
	profile.FieldDLSSRRMode:        "RR mode",
	profile.FieldDLSSRRPreset:      "RR preset",
	profile.FieldDLSSRROverride:    "RR override",
	profile.FieldDLSSFGEnabled:     "FG enabled",
	profile.FieldDLSSFGOverride:    "FG override",
	profile.FieldDLSSMultiFrame:    "Multi-frame",
	profile.FieldDLSSIndicator:     "SR indicator",
	profile.FieldDLSSFGIndicator:   "FG indicator",

	profile.FieldGPUClockOffset:          "Clock offset",
	profile.FieldGPUMemoryOffset:         "Memory offset",
	profile.FieldGPUPowerLimit:           "Power limit",
	profile.FieldGPUFanSpeed:             "Fan speed",
	profile.FieldGPUPowerMizer:           "Power mode",
	profile.FieldGPUShaderCache:          "Shader cache",
	profile.FieldGPUShaderCachePath:      "Shader cache path",
	profile.FieldGPUThreadedOptimization: "Threaded opt",

	profile.FieldCPUGovernor: "Governor",
	profile.FieldCPUSMT:      "SMT",
	profile.FieldCPUAffinity: "Affinity",

	profile.FieldOverlayEnabled:       "Enabled",
	profile.FieldOverlayPosition:      "Position",
	profile.FieldOverlayShowFPS:       "Show FPS",
	profile.FieldOverlayShowFrametime: "Show frametime",
	profile.FieldOverlayShowCPU:       "Show CPU",
	profile.FieldOverlayShowGPU:       "Show GPU",
	profile.FieldOverlayShowVRAM:      "Show VRAM",
	profile.FieldOverlayToggleKey:     "Toggle key",
}

// detailRow is one row in the flattened detail list. Either a group header
// (headerLabel set, field empty) or a field (field set, headerLabel empty).
type detailRow struct {
	headerLabel string // set for group header rows
	field       string // set for field rows (profile field key)
	label       string // display label for field rows
}

// isHeader reports whether the row is a group header (non-focusable).
func (r detailRow) isHeader() bool { return r.headerLabel != "" }

// DetailModel is the shared read-only profile detail renderer used by both
// the Games and Defaults resources. Navigation: j/k moves cursor through the
// focusable field rows, skipping group headers; the focused row renders with
// the accent-focus token. Actual field values come from ResolveForApply so
// Games view reflects inherited defaults without the caller pre-resolving.
//
// isRoot=true signals "this is the defaults profile root" — no inheritance
// markers, no reset/pin bindings offered. Task 5 consumes isRoot to gate the
// marker rendering.
type DetailModel struct {
	styles *Styles

	// raw is the profile as stored on disk (game or defaults). Task 5 reads
	// IsOverridden on this to decide marker rendering. In Task 4 we stash it
	// for structural symmetry but do not consult it during rendering.
	raw *profile.Profile

	// resolved is the effective profile for display — the output of
	// ResolveForApply(defaults) for a game profile, or a copy of defaults
	// itself when isRoot. Always non-nil after New*.
	resolved *profile.Profile

	// defaults is the defaults profile used to resolve raw. nil when isRoot.
	defaults *profile.Profile

	isRoot bool

	rows          []detailRow
	focusableRows []int // indices into rows
	cursor        int   // index into focusableRows (0..len(focusableRows)-1)
	width, height int
}

// NewDetail constructs a game-profile detail renderer. raw is the on-disk
// game profile (nil is safe — everything renders as inherited from defaults).
// defaults is the defaults profile that inherited fields fall back to.
func NewDetail(styles *Styles, raw, defaults *profile.Profile) DetailModel {
	return buildDetail(styles, raw, defaults, false)
}

// NewRootDetail constructs a defaults-root detail renderer. Renders the
// defaults profile itself with no inheritance markers and no reset/pin
// keybindings. Passing nil yields an empty-values rendering.
func NewRootDetail(styles *Styles, defaults *profile.Profile) DetailModel {
	return buildDetail(styles, defaults, nil, true)
}

func buildDetail(styles *Styles, raw, defaults *profile.Profile, isRoot bool) DetailModel {
	var resolved *profile.Profile
	switch {
	case isRoot:
		// Root: the profile IS the defaults. The raw values ARE the resolved
		// values — no inheritance chain to walk. ResolveForApply(nil) would
		// drop any field not marked Overridden, which is exactly wrong for
		// a root profile loaded from disk. Use the struct verbatim instead.
		if raw == nil {
			resolved = &profile.Profile{}
		} else {
			// Shallow copy so View() never mutates the caller's struct.
			copied := *raw
			resolved = &copied
		}
	default:
		if raw == nil {
			raw = &profile.Profile{}
		}
		resolved = raw.ResolveForApply(defaults)
	}

	rows, focusable := buildDetailRows()

	return DetailModel{
		styles:        styles,
		raw:           raw,
		resolved:      resolved,
		defaults:      defaults,
		isRoot:        isRoot,
		rows:          rows,
		focusableRows: focusable,
		cursor:        0,
	}
}

// buildDetailRows flattens the canonical subsystem order into alternating
// group-header and field rows, and returns the indices of the focusable
// (field) rows so j/k can step across header boundaries transparently.
func buildDetailRows() ([]detailRow, []int) {
	var rows []detailRow
	var focusable []int

	for _, section := range detailSectionOrder {
		title := detailSectionTitle[section]
		if title == "" {
			title = section
		}
		rows = append(rows, detailRow{headerLabel: title})
		for _, field := range profile.SectionFields(section) {
			label := fieldLabels[field]
			if label == "" {
				label = field
			}
			focusable = append(focusable, len(rows))
			rows = append(rows, detailRow{field: field, label: label})
		}
	}

	return rows, focusable
}

// SetSize stores the render dimensions.
func (m *DetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Cursor reports the current focus cursor — an index into the focusable
// field list (0 = first field, len(focusableRows)-1 = last field). Useful
// for tests and for parent models that want to persist selection.
func (m DetailModel) Cursor() int { return m.cursor }

// IsRoot reports whether this renderer is displaying a root defaults profile
// (suppresses inheritance markers, no reset/pin bindings).
func (m DetailModel) IsRoot() bool { return m.isRoot }

// FocusedField returns the profile field key of the currently focused row,
// or "" if there are no focusable rows.
func (m DetailModel) FocusedField() string {
	if len(m.focusableRows) == 0 {
		return ""
	}
	idx := m.focusableRows[m.cursor]
	return m.rows[idx].field
}

// FieldCount returns the number of focusable field rows.
func (m DetailModel) FieldCount() int { return len(m.focusableRows) }

// Update handles cursor navigation. Contract:
//
//   - "j" / "down" moves focus to the next field, crossing group-header
//     boundaries transparently (clamped at the last field).
//   - "k" / "up"   moves focus to the previous field (clamped at zero).
//
// All other keys pass through (return handled=false) so the caller can route
// them elsewhere — for example the pane's parent handlers for tab/esc, or
// the Games-resource r/shift+r/p reset/pin bindings that live on the caller.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}
	switch key.String() {
	case "j", "down":
		if m.cursor < len(m.focusableRows)-1 {
			m.cursor++
		}
		return m, nil, true
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil, true
	}
	return m, nil, false
}

// IsOverridden reports whether the currently focused field is an override on
// the raw profile. Returns false for root profiles (inheritance markers are
// suppressed there) or when there is no focused row.
func (m DetailModel) IsOverridden(field string) bool {
	if m.isRoot || m.raw == nil {
		return false
	}
	return m.raw.IsOverridden(field)
}

// ResetFocused resets the currently focused field to inherited on the raw
// profile and rebuilds the resolved view. Returns (changed, error). A no-op
// (changed=false) when the field is already inherited, when no field is
// focused, or when the renderer is in root mode. Task 5 consumes this for
// the `r` binding.
func (m *DetailModel) ResetFocused() (bool, error) {
	if m.isRoot || m.raw == nil {
		return false, nil
	}
	field := m.FocusedField()
	if field == "" {
		return false, nil
	}
	if !m.raw.IsOverridden(field) {
		return false, nil
	}
	if err := m.raw.Reset(field); err != nil {
		return false, err
	}
	m.rebuildResolved()
	return true, nil
}

// ResetAll resets every field on the raw profile to inherited and rebuilds
// the resolved view. Returns true when at least one override was cleared.
// No-op and returns false when in root mode. Task 5 consumes this for the
// `shift+r` / `R` binding.
func (m *DetailModel) ResetAll() bool {
	if m.isRoot || m.raw == nil {
		return false
	}
	if len(m.raw.Overrides) == 0 {
		// Still zero the struct in case the raw has stray values without
		// override flags — but report no change so callers can skip saving.
		return false
	}
	m.raw.ResetAll()
	m.rebuildResolved()
	return true
}

// PinFocused pins the currently-resolved value of the focused field as an
// override on the raw profile. Reads the effective value from the resolved
// profile (which already accounts for defaults inheritance) so the pin
// captures exactly what the user sees. No-op when the field is already
// overridden, when no field is focused, or when in root mode. Returns
// (changed, error). Task 5 consumes this for the `p` binding.
func (m *DetailModel) PinFocused() (bool, error) {
	if m.isRoot || m.raw == nil {
		return false, nil
	}
	field := m.FocusedField()
	if field == "" {
		return false, nil
	}
	if m.raw.IsOverridden(field) {
		return false, nil
	}
	if err := m.raw.PinField(field, m.defaults); err != nil {
		return false, err
	}
	m.rebuildResolved()
	return true, nil
}

// RawProfile returns the underlying raw profile pointer (mutated by reset/
// pin operations). Callers use this to feed the save pipeline after a
// binding fires. nil when the renderer has no backing profile.
func (m DetailModel) RawProfile() *profile.Profile {
	return m.raw
}

// rebuildResolved recomputes the resolved view after raw is mutated. For a
// game profile we re-run ResolveForApply; for a root profile the raw IS
// the resolved view (no inheritance chain to walk).
func (m *DetailModel) rebuildResolved() {
	if m.raw == nil {
		m.resolved = &profile.Profile{}
		return
	}
	if m.isRoot {
		copied := *m.raw
		m.resolved = &copied
		return
	}
	m.resolved = m.raw.ResolveForApply(m.defaults)
}

// overrideMarkerGlyph is the single-character marker rendered next to an
// overridden field in the Games detail view. Task 5 picks the diamond so it
// lines up with the ◆ already used in the games sidebar to indicate "game
// has a profile", reading as a consistent family of override signals. Rendered
// in AccentOverride (magenta) via OverrideMarkerStyle. Root profiles never
// render this marker (isRoot suppresses all inheritance rendering).
const overrideMarkerGlyph = "◆"

// View renders the detail as a single column: for each group, a bold header
// line then one row per field formatted as `  marker label  value`.
//
// Per-row styling (Task 5):
//   - Focused row wins: rendered with FocusStyle (accent-focus cyan, bold).
//   - Otherwise, in a game-profile view (isRoot=false): overridden fields
//     render in OverrideStyle (fg) with a magenta ◆ marker; inherited fields
//     render in InheritedStyle (fg-muted) with no marker.
//   - In the root defaults view (isRoot=true), all fields render in Normal
//     style with no marker — matching the Task 4 acceptance.
func (m DetailModel) View() string {
	s := m.styles
	if s == nil {
		return ""
	}

	var b strings.Builder
	focusedRow := -1
	if len(m.focusableRows) > 0 {
		focusedRow = m.focusableRows[m.cursor]
	}

	for i, row := range m.rows {
		if row.isHeader() {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(s.Title.Render(row.headerLabel))
			b.WriteString("\n")
			continue
		}

		value := formatFieldValue(m.resolved, row.field)
		overridden := !m.isRoot && m.raw != nil && m.raw.IsOverridden(row.field)

		marker := "  "
		if overridden {
			marker = s.OverrideMarkerStyle().Render(overrideMarkerGlyph) + " "
		}
		body := fmt.Sprintf("%-20s  %s", row.label, value)

		if i == focusedRow {
			// Focus styling applies to the whole row body (the marker keeps
			// its own magenta so the override signal stays readable on top
			// of the focus highlight).
			body = s.FocusStyle().Render(body)
		} else if overridden {
			body = s.OverrideStyle().Render(body)
		} else if !m.isRoot {
			body = s.InheritedStyle().Render(body)
		} else {
			body = s.Normal.Render(body)
		}
		b.WriteString("  ")
		b.WriteString(marker)
		b.WriteString(body)
		b.WriteString("\n")
	}

	return b.String()
}

// formatFieldValue returns a display string for the given field on the
// resolved profile. Handles every field type that appears in profile.Profile
// (bool, int, string, DLSS enums, CPU SMT *bool). Returns "(default)" for
// zero values so the renderer reads naturally for an unset field.
func formatFieldValue(p *profile.Profile, field string) string {
	if p == nil {
		return "(default)"
	}
	switch field {
	case profile.FieldProtonEnableHDR:
		return displayBool(p.Proton.EnableHDR)
	case profile.FieldProtonEnableWayland:
		return displayBool(p.Proton.EnableWayland)
	case profile.FieldProtonEnableNGXUpdater:
		return displayBool(p.Proton.EnableNGXUpdater)
	case profile.FieldProtonVKD3DHeap:
		return displayBool(p.Proton.VKD3DHeap)

	case profile.FieldDLSSSRMode:
		return displayValue(string(p.DLSS.SRMode))
	case profile.FieldDLSSSRPreset:
		return displayValue(string(p.DLSS.SRPreset))
	case profile.FieldDLSSSRModelPreset:
		return displayValue(string(p.DLSS.SRModelPreset))
	case profile.FieldDLSSSROverride:
		return displayBool(p.DLSS.SROverride)
	case profile.FieldDLSSRRMode:
		return displayValue(string(p.DLSS.RRMode))
	case profile.FieldDLSSRRPreset:
		return displayValue(string(p.DLSS.RRPreset))
	case profile.FieldDLSSRROverride:
		return displayBool(p.DLSS.RROverride)
	case profile.FieldDLSSFGEnabled:
		return displayBool(p.DLSS.FGEnabled)
	case profile.FieldDLSSFGOverride:
		return displayBool(p.DLSS.FGOverride)
	case profile.FieldDLSSMultiFrame:
		return displayInt(p.DLSS.MultiFrame)
	case profile.FieldDLSSIndicator:
		return displayBool(p.DLSS.Indicator)
	case profile.FieldDLSSFGIndicator:
		return displayBool(p.DLSS.FGIndicator)

	case profile.FieldGPUClockOffset:
		return displayInt(p.GPU.ClockOffset)
	case profile.FieldGPUMemoryOffset:
		return displayInt(p.GPU.MemoryOffset)
	case profile.FieldGPUPowerLimit:
		return displayInt(p.GPU.PowerLimit)
	case profile.FieldGPUFanSpeed:
		return displayInt(p.GPU.FanSpeed)
	case profile.FieldGPUPowerMizer:
		return displayValue(p.GPU.PowerMizer)
	case profile.FieldGPUShaderCache:
		return displayBool(p.GPU.ShaderCache)
	case profile.FieldGPUShaderCachePath:
		return displayValue(p.GPU.ShaderCachePath)
	case profile.FieldGPUThreadedOptimization:
		return displayBool(p.GPU.ThreadedOptimization)

	case profile.FieldCPUGovernor:
		return displayValue(p.CPU.Governor)
	case profile.FieldCPUSMT:
		return displayBoolPtr(p.CPU.SMT)
	case profile.FieldCPUAffinity:
		return displayValue(p.CPU.Affinity)

	case profile.FieldOverlayEnabled:
		return displayBool(p.Overlay.Enabled)
	case profile.FieldOverlayPosition:
		return displayValue(p.Overlay.Position)
	case profile.FieldOverlayShowFPS:
		return displayBool(p.Overlay.ShowFPS)
	case profile.FieldOverlayShowFrametime:
		return displayBool(p.Overlay.ShowFrametime)
	case profile.FieldOverlayShowCPU:
		return displayBool(p.Overlay.ShowCPU)
	case profile.FieldOverlayShowGPU:
		return displayBool(p.Overlay.ShowGPU)
	case profile.FieldOverlayShowVRAM:
		return displayBool(p.Overlay.ShowVRAM)
	case profile.FieldOverlayToggleKey:
		return displayValue(p.Overlay.ToggleKey)
	}
	return "(default)"
}
