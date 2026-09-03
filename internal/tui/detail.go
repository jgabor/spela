// Package tui contains the terminal user interface.
package tui

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jgabor/spela/internal/profile"
)

// detailSectionOrder is the canonical rendering order for subsystem groups.
var detailSectionOrder = []string{"proton", "dlss", "gpu", "cpu", "overlay"}

// detailSectionTitle maps a subsystem key to its human-readable header.
var detailSectionTitle = map[string]string{
	"proton":  "Proton",
	"dlss":    "DLSS",
	"gpu":     "GPU",
	"cpu":     "CPU",
	"overlay": "Overlay",
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

// DetailModel is the shared profile detail renderer used by both
// the Games and Defaults resources. Navigation: j/k moves cursor through the
// focusable field rows, skipping group headers; the focused row renders with
// the accent-focus token. Actual field values come from ResolveForApply so
// Games view reflects inherited defaults without the caller pre-resolving.
//
// isRoot=true signals "this is the defaults profile root" — no inheritance
// markers or pin bindings are offered.
type DetailModel struct {
	styles *Styles

	// raw is the profile as stored on disk (game or defaults).
	raw *profile.Profile
	// persisted is the last successfully saved baseline. raw is an independent
	// draft and may diverge until Save or Cancel.
	persisted *profile.Profile

	// resolved is the effective profile for display — the output of
	// ResolveForApply(defaults) for a game profile, or a copy of defaults
	// itself when isRoot. Always non-nil after New*.
	resolved *profile.Profile

	// defaults is the defaults profile used to resolve raw. nil when isRoot.
	defaults *profile.Profile

	isRoot    bool
	editor    EditorHost
	saveError error

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
	if raw == nil {
		raw = &profile.Profile{}
	}
	persisted := raw.Clone()
	var resolved *profile.Profile
	switch {
	case isRoot:
		// Root: the profile IS the defaults. The raw values ARE the resolved
		// values — no inheritance chain to walk. ResolveForApply(nil) would
		// drop any field not marked Overridden, which is exactly wrong for
		// a root profile loaded from disk. Use the struct verbatim instead.
		resolved = raw.Clone()
	default:
		resolved = raw.ResolveForApply(defaults)
	}

	rows, focusable := buildDetailRowsFiltered("")

	return DetailModel{
		styles:        styles,
		raw:           raw,
		persisted:     persisted,
		resolved:      resolved,
		defaults:      defaults,
		isRoot:        isRoot,
		rows:          rows,
		focusableRows: focusable,
		cursor:        0,
	}
}

// Editing reports whether the focused profile value currently owns input.
func (m DetailModel) Editing() bool { return m.editor.Active() }

// Dirty reports whether the draft differs from the last saved profile.
func (m DetailModel) Dirty() bool { return !reflect.DeepEqual(m.raw, m.persisted) }

// SaveError is retained until the draft is changed, cancelled, or saved.
func (m DetailModel) SaveError() error { return m.saveError }

// BeginEdit opens the shared editor host for the focused field. Inherited game
// fields start from their effective value; committing therefore creates a
// concrete override.
func (m *DetailModel) BeginEdit() bool {
	field := m.FocusedField()
	descriptor, ok := profile.Field(field)
	if !ok {
		return false
	}
	value, err := profile.ReadField(m.resolved, field)
	if err != nil {
		return false
	}
	m.editor.Begin(profileEditorSpec(descriptor), profileEditorValue(value))
	m.saveError = nil
	return true
}

// UpdateEditor applies input to the active editor. Enter commits to the draft;
// Escape restores the field's pre-edit value.
func (m *DetailModel) UpdateEditor(key tea.KeyPressMsg) bool {
	if !m.editor.Active() {
		return false
	}
	switch key.String() {
	case "esc":
		m.editor.Cancel()
		return true
	case "enter", "ctrl+s":
		value, err := m.editor.Commit()
		if err != nil {
			return true
		}
		if err := m.setFocusedEditorValue(value); err != nil {
			m.editor.Begin(profileEditorSpec(m.focusedDescriptor()), value)
			m.editor.err = err
			return true
		}
		m.saveError = nil
		m.rebuildResolved()
		return true
	case "backspace":
		m.editor.Delete()
		return true
	case "left":
		m.editor.Cycle(-1)
		return true
	case "right":
		m.editor.Cycle(1)
		return true
	}
	if key.Text != "" {
		m.editor.Append(key.Text)
		return true
	}
	return true
}

func (m DetailModel) focusedDescriptor() profile.FieldDescriptor {
	descriptor, _ := profile.Field(m.FocusedField())
	return descriptor
}

func profileEditorSpec(descriptor profile.FieldDescriptor) EditorSpec {
	kind := EditorText
	switch descriptor.Editor {
	case profile.EditorToggle:
		kind = EditorBool
	case profile.EditorChoice:
		kind = EditorChoice
	case profile.EditorInteger:
		kind = EditorInteger
	}
	return EditorSpec{Key: descriptor.Key, Kind: kind, Choices: append([]string(nil), descriptor.AllowedValues...)}
}

func profileEditorValue(value any) string {
	switch value := value.(type) {
	case bool:
		return strconv.FormatBool(value)
	case int:
		return strconv.Itoa(value)
	case string:
		return value
	case *bool:
		if value != nil {
			return strconv.FormatBool(*value)
		}
	}
	return ""
}

func (m *DetailModel) setFocusedEditorValue(value string) error {
	descriptor := m.focusedDescriptor()
	var typed any = value
	switch descriptor.Kind {
	case profile.PrimitiveBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s", value)
		}
		typed = parsed
	case profile.PrimitiveInt:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer: %s", value)
		}
		typed = parsed
	case profile.PrimitiveOptionalBool:
		if value == "" {
			typed = (*bool)(nil)
		} else {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean: %s", value)
			}
			typed = &parsed
		}
	}
	return m.raw.Set(descriptor.Key, typed)
}

// CancelDraft discards all unsaved profile changes.
func (m *DetailModel) CancelDraft() {
	if m.persisted != nil {
		m.raw = m.persisted.Clone()
	}
	m.editor.Cancel()
	m.saveError = nil
	m.rebuildResolved()
}

// CompleteSave advances the baseline on success or retains the draft and error
// for an in-place retry on failure.
func (m *DetailModel) CompleteSave(err error) {
	if err != nil {
		m.saveError = err
		return
	}
	m.persisted = m.raw.Clone()
	m.saveError = nil
}

// buildDetailRowsFiltered flattens subsystem groups into rows. When sectionKey
// is non-empty, only that section is included.
func buildDetailRowsFiltered(sectionKey string) ([]detailRow, []int) {
	var rows []detailRow
	var focusable []int

	for _, section := range detailSectionOrder {
		if sectionKey != "" && section != sectionKey {
			continue
		}
		title := detailSectionTitle[section]
		if title == "" {
			title = section
		}
		rows = append(rows, detailRow{headerLabel: title})
		for _, field := range profile.SectionFields(section) {
			descriptor, _ := profile.Field(field)
			label := descriptor.Label
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

// CycleFocusedField cycles the focused field on a root defaults profile.
// Bool fields rotate through "(default)", "true", and "false". Returns true
// when the value changed.
func (m *DetailModel) CycleFocusedField(direction int) bool {
	if !m.isRoot || m.raw == nil {
		return false
	}
	field := m.FocusedField()
	if field == "" {
		return false
	}

	options := rootFieldOptions(field)
	if len(options) == 0 {
		return false
	}

	current := formatRootFieldValue(m.resolved, field)
	idx := 0
	for i, opt := range options {
		if opt == current {
			idx = i
			break
		}
	}
	step := 1
	if direction < 0 {
		step = -1
	}
	next := options[(idx+step+len(options))%len(options)]
	if next == current {
		return false
	}
	if !applyRootFieldValue(m.raw, field, next) {
		return false
	}
	m.rebuildResolved()
	return true
}

// RestoreFocus moves the cursor back to a field (by key) or index after the
// detail model is rebuilt — e.g. after a save reload.
func (m *DetailModel) RestoreFocus(field string, cursor int) {
	if field != "" {
		for i, rowIdx := range m.focusableRows {
			if m.rows[rowIdx].field == field {
				m.cursor = i
				return
			}
		}
	}
	if len(m.focusableRows) > 0 {
		m.cursor = max(min(cursor, len(m.focusableRows)-1), 0)
	}
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

// ResetFocused resets the currently focused field. On a game profile it clears
// an override back to inherited; on the root defaults profile it clears the
// field back to "(default)".
func (m *DetailModel) ResetFocused() (bool, error) {
	if m.raw == nil {
		return false, nil
	}
	field := m.FocusedField()
	if field == "" {
		return false, nil
	}
	if m.isRoot {
		if m.rootFieldAtDefault(field) {
			return false, nil
		}
	} else if !m.raw.IsOverridden(field) {
		return false, nil
	}
	if err := m.raw.Reset(field); err != nil {
		return false, err
	}
	m.rebuildResolved()
	return true, nil
}

// ResetAll resets every field on the profile. On a game profile only overrides
// are cleared; on the root defaults profile every field returns to "(default)".
func (m *DetailModel) ResetAll() bool {
	if m.raw == nil {
		return false
	}
	if m.isRoot {
		if m.rootProfileAtDefault() {
			return false
		}
		m.raw.ResetAll()
		m.rebuildResolved()
		return true
	}
	if len(m.raw.Overrides) == 0 {
		return false
	}
	m.raw.ResetAll()
	m.rebuildResolved()
	return true
}

// RawProfile returns the current in-memory draft. Callers clone it before
// passing it to the save pipeline.
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
// overridden field in the Games detail view. The diamond lines up with the ◆
// already used in the games sidebar to indicate "game
// has a profile", reading as a consistent family of override signals. Rendered
// in AccentOverride (magenta) via OverrideMarkerStyle. Root profiles never
// render this marker (isRoot suppresses all inheritance rendering).
const overrideMarkerGlyph = "◆"

// View renders the detail as a single column: for each group, a bold header
// line then one row per field formatted as `  marker label  value  semantics`.
//
// Per-row styling:
//   - Focused row wins: rendered with FocusStyle (accent-focus cyan, bold).
//   - Otherwise, in a game-profile view (isRoot=false): overridden fields
//     render in OverrideStyle (fg) with a magenta ◆ marker; inherited fields
//     render in InheritedStyle (fg-muted) with no marker.
//   - In the root defaults view (isRoot=true), all fields render in Normal
//     style with no marker.
func (m DetailModel) View() string {
	s := m.styles
	if s == nil {
		return ""
	}

	var lines []string
	focusedRow := -1
	if len(m.focusableRows) > 0 {
		focusedRow = m.focusableRows[m.cursor]
	}
	if m.editor.Error() != nil {
		lines = append(lines, s.Error.Render("Invalid value: "+m.editor.Error().Error()))
	}
	if m.saveError != nil {
		lines = append(lines, s.Error.Render("Save failed: "+m.saveError.Error()))
	}
	if m.Dirty() {
		lines = append(lines, s.Warning.Render("Unsaved changes  s:save  Esc:cancel"))
	}

	focusedLabel := ""
	for i, row := range m.rows {
		if row.isHeader() {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, s.Title.Render(row.headerLabel))
			continue
		}

		overridden := !m.isRoot && m.raw != nil && m.raw.IsOverridden(row.field)
		value := formatFieldValue(m.resolved, row.field)
		if m.isRoot {
			value = formatRootFieldValue(m.resolved, row.field)
		} else if overridden {
			value = formatExplicitFieldValue(m.raw, row.field)
		}
		if i == focusedRow && m.editor.Active() {
			value = m.editor.Value()
		}
		semantics := m.formatFieldState(row.field, overridden)

		marker := "  "
		if i == focusedRow {
			if overridden {
				marker = "> " + s.OverrideMarkerStyle().Render(overrideMarkerGlyph) + " "
			} else {
				marker = "> "
			}
		} else if overridden {
			marker = s.OverrideMarkerStyle().Render(overrideMarkerGlyph) + " "
		}
		body := fmt.Sprintf("%-20s  %-12s  %s", row.label, value, semantics)
		if i == focusedRow && m.editor.Active() {
			body += s.Dim.Render("  [Enter apply · Esc cancel]")
		} else if i == focusedRow && !m.isRoot {
			body += s.Dim.Render("  [Enter edit · r Inherited · Shift+R all]")
		} else if i == focusedRow && m.isRoot {
			body += s.Dim.Render("  [Enter edit · r System default]")
		}
		if m.width > 0 {
			body = ansi.Truncate(body, max(m.width-7, 1), "…")
		}

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
		if i == focusedRow {
			focusedLabel = row.label
		}
		lines = append(lines, "  "+marker+body)
		if i == focusedRow {
			explanation := "    Effect: " + m.formatFieldSemantics(row.field)
			if m.width > 0 {
				explanation = ansi.Truncate(explanation, max(m.width-4, 1), "…")
			}
			lines = append(lines, s.Dim.Render(explanation))
		}
	}
	lines = strings.Split(strings.Join(lines, "\n"), "\n")
	focusedLine := 0
	for index, line := range lines {
		if focusedLabel != "" && strings.Contains(ansi.Strip(line), focusedLabel) {
			focusedLine = index
			break
		}
	}
	if m.height > 0 && len(lines) > m.height {
		start := max(focusedLine-m.height+1, 0)
		start = min(start, len(lines)-m.height)
		lines = lines[start : start+m.height]
	}
	return strings.Join(lines, "\n")
}

func (m DetailModel) formatFieldSemantics(field string) string {
	descriptor, ok := profile.Field(field)
	if !ok {
		return ""
	}
	if m.isRoot {
		return fieldEffect(descriptor)
	}
	raw := m.raw
	if raw == nil {
		raw = &profile.Profile{}
	}
	explanation, err := raw.ExplainField(field, m.defaults)
	if err != nil {
		return fieldEffect(descriptor)
	}
	return fieldEffect(profile.FieldDescriptor{Impact: explanation.Impact, Restore: explanation.Restore})
}

func (m DetailModel) formatFieldState(_ string, overridden bool) string {
	if m.isRoot {
		return "○ Default for all games"
	}
	if overridden {
		return "Override for this game"
	}
	return "↳ Inherited from defaults"
}

func fieldEffect(descriptor profile.FieldDescriptor) string {
	effect := "Applied when the game starts"
	switch descriptor.Impact {
	case profile.LaunchImpactSystemState:
		effect = "Changes system settings while the game runs"
	case profile.LaunchImpactOverlay:
		effect = "Changes the in-game overlay"
	}
	restore := "ends with the game"
	switch descriptor.Restore {
	case profile.RestoreCoverageRestorableMutation:
		restore = "restored when the game exits"
	case profile.RestoreCoverageNotApplicable:
		restore = "no system setting to restore"
	}
	return effect + "; " + restore
}

func rootFieldOptions(field string) []string {
	if descriptor, ok := profile.Field(field); ok && descriptor.Kind == profile.PrimitiveBool {
		return []string{"(default)", "true", "false"}
	}
	return nil
}

func applyRootFieldValue(p *profile.Profile, field, value string) bool {
	descriptor, ok := profile.Field(field)
	if !ok || descriptor.Kind != profile.PrimitiveBool {
		return false
	}
	if value == "(default)" {
		return p.Reset(field) == nil
	}
	return p.Set(field, value == "true") == nil
}

func formatRootBoolField(p *profile.Profile, field string) string {
	if p == nil {
		return "(default)"
	}
	value, err := profile.ReadField(p, field)
	val, isBool := value.(bool)
	if err != nil || !isBool {
		return "(default)"
	}
	if !p.IsOverridden(field) {
		if !val {
			return "(default)"
		}
		return "true"
	}
	if val {
		return "true"
	}
	return "false"
}

func (m DetailModel) rootFieldAtDefault(field string) bool {
	if m.raw == nil {
		return true
	}
	return formatRootFieldValue(m.raw, field) == "(default)"
}

func (m DetailModel) rootProfileAtDefault() bool {
	if m.raw == nil {
		return true
	}
	for _, field := range profile.AllFields() {
		if !m.rootFieldAtDefault(field) {
			return false
		}
	}
	return true
}

// formatRootFieldValue renders defaults-profile field values. Bool fields
// distinguish "(default)" (unset/zero) from explicit "true"/"false".
func formatRootFieldValue(p *profile.Profile, field string) string {
	if p != nil && p.IsOverridden(field) {
		return formatExplicitFieldValue(p, field)
	}
	if descriptor, ok := profile.Field(field); ok && descriptor.Kind == profile.PrimitiveBool {
		return formatRootBoolField(p, field)
	}
	return formatFieldValue(p, field)
}

func formatExplicitFieldValue(p *profile.Profile, field string) string {
	value, err := profile.ReadField(p, field)
	if err != nil {
		return "(unset)"
	}
	if value == nil {
		return "(no value)"
	}
	switch value := value.(type) {
	case bool:
		return fmt.Sprint(value)
	case int:
		return fmt.Sprint(value)
	case string:
		if value == "" {
			return "(empty)"
		}
		return value
	default:
		return fmt.Sprint(value)
	}
}

// formatFieldValue returns a display string for the given field on the
// resolved profile. Handles every field type that appears in profile.Profile
// (bool, int, string, DLSS enums, CPU SMT *bool). Returns "(default)" for
// zero values so the renderer reads naturally for an unset field.
func formatFieldValue(p *profile.Profile, field string) string {
	if p == nil {
		return "(default)"
	}
	descriptor, ok := profile.Field(field)
	if !ok {
		return "(default)"
	}
	value, err := profile.ReadField(p, descriptor.Key)
	if err != nil || value == nil {
		return "(default)"
	}
	switch value := value.(type) {
	case bool:
		if descriptor.Kind == profile.PrimitiveOptionalBool {
			return fmt.Sprint(value)
		}
		if !value {
			return "(default)"
		}
		return "true"
	case int:
		if value == 0 {
			return "(default)"
		}
		return fmt.Sprint(value)
	case string:
		if value == "" || value == "default" || value == "auto" {
			return "(default)"
		}
		return value
	}
	return "(default)"
}
