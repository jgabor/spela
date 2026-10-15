// Package tui contains the terminal user interface.
package tui

import (
	"fmt"
	"reflect"
	"slices"
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
// the Games and Defaults resources. Navigation: arrows move the cursor through the
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
	// working copy; it runs ahead of persisted while a write is in flight.
	persisted *profile.Profile

	// resolved is the effective profile for display — the output of
	// ResolveForApply(defaults) for a game profile, or a copy of defaults
	// itself when isRoot. Always non-nil after New*.
	resolved *profile.Profile

	// defaults is the defaults profile used to resolve raw. nil when isRoot.
	defaults *profile.Profile

	isRoot bool
	// ownValueGames lists, per field, the games that set their own value. It is
	// only used on the All games page, where it gives the count and the action
	// that applies All games to those games.
	ownValueGames map[string][]string
	editor        EditorHost
	saveError     error
	// lastChange names the most recent change for the message bar.
	lastChange string
	// lastOwn remembers, per field, the value a game had before Space made it
	// follow All games, so the next Space puts it back.
	lastOwn map[string]string

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

// Dirty reports whether a change has not reached the file yet: a write is in
// flight or the last write failed. Changes are saved as they are made.
func (m DetailModel) Dirty() bool { return profilesDiffer(m.raw, m.persisted) }

// profilesDiffer compares two profiles the way a save would: an absent override
// and a false map entry both mean inheritance, while a true pin is kept even
// when its value is false or zero.
func profilesDiffer(left, right *profile.Profile) bool {
	var first, second profile.Profile
	if left != nil {
		first = *left
	}
	if right != nil {
		second = *right
	}
	firstOverrides, secondOverrides := first.Overrides, second.Overrides
	first.Overrides, second.Overrides = nil, nil
	if !reflect.DeepEqual(first, second) {
		return true
	}
	for field, overridden := range firstOverrides {
		if overridden && !secondOverrides[field] {
			return true
		}
	}
	for field, overridden := range secondOverrides {
		if overridden && !firstOverrides[field] {
			return true
		}
	}
	return false
}

// SaveError is retained until the next change.
func (m DetailModel) SaveError() error { return m.saveError }

// BeginEdit opens the editor for the focused field: a picker for booleans and
// fields with a fixed set of values, a text editor for numbers and text.
// Inherited fields start from their effective value; applying one creates a
// concrete value for this page.
func (m *DetailModel) BeginEdit() bool {
	field := m.FocusedField()
	descriptor, ok := profile.Field(field)
	if !ok {
		return false
	}
	if options := fieldCycleOptions(field); len(options) > 0 {
		current := cycleFollow
		if m.isOwn(field) {
			value, err := profile.ReadField(m.raw, field)
			if err != nil {
				return false
			}
			current = profileEditorValue(value)
		}
		if !slices.Contains(options, current) {
			current = options[0]
		}
		m.editor.Begin(EditorSpec{Key: field, Kind: EditorChoice, Choices: options, Labels: m.choiceLabels(descriptor, options)}, current)
		m.saveError = nil
		return true
	}
	value, err := profile.ReadField(m.resolved, field)
	if err != nil {
		return false
	}
	m.editor.Begin(profileEditorSpec(descriptor), profileEditorValue(value))
	m.saveError = nil
	return true
}

// EditorInputFocused reports whether the value input owns printable keys.
func (m DetailModel) EditorInputFocused() bool         { return m.editor.InputFocused() }
func (m DetailModel) EditorInputKind() EditorValueKind { return m.editor.Kind() }
func (m DetailModel) EditorEnterLabel() string         { return m.editor.EnterLabel() }

// UpdateEditorInput cannot invoke an application command. Text editing stays
// inside the active Input control and never reaches an editor button.
func (m *DetailModel) UpdateEditorInput(key tea.KeyPressMsg) bool {
	return m.editor.UpdateInput(key)
}

// UpdateAction applies one action. changed is true when the action altered the
// profile; the owning pipeline then saves it at once and reads LastChange for
// the message. handled is true when the action belonged to this page.
func (m *DetailModel) UpdateAction(action KeyAction) (changed bool, handled bool) {
	if m.Editing() {
		switch action {
		case ActionFocusNext:
			m.editor.FocusNext()
		case ActionEditCancel:
			m.editor.Cancel()
		case ActionEditCommit:
			if m.editor.focus == EditorFocusCancel {
				m.editor.Cancel()
				return false, true
			}
			return m.applyEditor(), true
		case ActionEditDelete:
			if m.EditorInputFocused() {
				m.editor.Delete()
			}
		case ActionDetailDecrease:
			m.editor.Move(-1)
		case ActionDetailIncrease:
			m.editor.Move(1)
		case ActionDetailPreviousItem:
			if m.EditorInputFocused() {
				m.editor.Cycle(-1)
			}
		case ActionDetailNextItem:
			if m.EditorInputFocused() {
				m.editor.Cycle(1)
			}
		case ActionEditToggle:
			m.editor.Toggle()
		default:
			return false, false
		}
		return false, true
	}
	switch action {
	case ActionDetailPreviousItem, ActionListPrevious:
		m.cursor = max(m.cursor-1, 0)
	case ActionDetailNextItem, ActionListNext:
		m.cursor = max(min(m.cursor+1, len(m.focusableRows)-1), 0)
	case ActionDetailConfirm:
		m.BeginEdit()
	case ActionDetailCycle:
		return m.ToggleFocusedField(), true
	case ActionDetailReset:
		reset, err := m.ResetFocused()
		m.saveError = err
		return reset, true
	case ActionDetailResetAll:
		cleared := m.ResetAll()
		m.saveError = nil
		return cleared, true
	default:
		return false, false
	}
	return false, true
}

// LastChange describes the most recent change, such as "HDR On".
func (m DetailModel) LastChange() string { return m.lastChange }

// noteChange records what a change did, in the words of the page.
func (m *DetailModel) noteChange(field string) {
	descriptor, ok := profile.Field(field)
	if !ok {
		return
	}
	value, _, own := m.fieldText(field)
	if m.isRoot {
		own = !m.rootFieldAtDefault(field)
	}
	switch {
	case own:
		m.lastChange = descriptor.Label + " " + value
	case m.isRoot:
		m.lastChange = descriptor.Label + " uses the system default"
	default:
		m.lastChange = descriptor.Label + " follows All games"
	}
	if m.isRoot {
		m.lastChange += " (" + profile.AllGamesName + ")"
	}
}

func (m *DetailModel) applyEditor() bool {
	value, err := m.editor.Commit()
	if err != nil {
		return false
	}
	before := m.raw.Clone()
	if err := m.setFocusedEditorValue(value); err != nil {
		m.editor.active = true
		m.editor.focus = EditorFocusInput
		m.editor.err = err
		return false
	}
	m.saveError = nil
	m.rebuildResolved()
	if !profilesDiffer(before, m.raw) {
		return false
	}
	m.noteChange(m.FocusedField())
	return true
}

// UpdateEditor is retained for input consumers that do not own persistence.
// Save requests are routed through UpdateAction by the document owner.
func (m *DetailModel) UpdateEditor(key tea.KeyPressMsg) bool {
	if !m.Editing() {
		return false
	}
	var action KeyAction
	switch key.String() {
	case "enter":
		action = ActionEditCommit
	case "tab":
		action = ActionFocusNext
	case "left":
		action = ActionDetailDecrease
	case "right":
		action = ActionDetailIncrease
	case "up":
		action = ActionDetailPreviousItem
	case "down":
		action = ActionDetailNextItem
	case "space":
		action = ActionEditToggle
	default:
		return m.UpdateEditorInput(key)
	}
	_, handled := m.UpdateAction(action)
	return handled
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
	if value == cycleFollow {
		return m.raw.Reset(descriptor.Key)
	}
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
			typed = nil
		} else {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean: %s", value)
			}
			typed = parsed
		}
	}
	return m.raw.Set(descriptor.Key, typed)
}

// RevertToPersisted returns the working copy to the last saved profile after a
// write failed, so the rows show what is stored.
func (m *DetailModel) RevertToPersisted() {
	if m.persisted != nil {
		m.raw = m.persisted.Clone()
	}
	m.rebuildResolved()
}

// CompleteSave advances the baseline on success or retains the change and error
// for an in-place retry on failure.
func (m *DetailModel) CompleteSave(err error) {
	m.CompleteSaveSnapshot(m.raw, err)
}

// CompleteSaveSnapshot advances only the saved baseline. A completion for an
// earlier snapshot cannot mark a later edit saved or replace its value.
func (m *DetailModel) CompleteSaveSnapshot(saved *profile.Profile, err error) {
	if err != nil {
		m.saveError = err
		return
	}
	m.persisted = saved.Clone()
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

// Update accepts basic cursor keys for standalone detail consumers. Shell
// routing uses UpdateAction so focus and availability are resolved once.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}
	var action KeyAction
	switch key.String() {
	case "down":
		action = ActionDetailNextItem
	case "up":
		action = ActionDetailPreviousItem
	case "space":
		action = ActionDetailCycle
	default:
		return m, nil, false
	}
	_, handled := m.UpdateAction(action)
	return m, nil, handled
}

// CanToggleFocusedField reports whether Space can act on the focused row.
func (m DetailModel) CanToggleFocusedField() bool {
	return m.raw != nil && m.FocusedField() != ""
}

// isOwn reports whether a field holds a value set on this page itself, as
// against following All games (the system default on the All games page).
func (m DetailModel) isOwn(field string) bool {
	if m.raw == nil || field == "" {
		return false
	}
	if m.isRoot {
		return !m.rootFieldAtDefault(field)
	}
	return m.raw.IsOverridden(field)
}

// FocusedIsOwn reports whether the focused field holds a value of its own.
func (m DetailModel) FocusedIsOwn() bool { return m.isOwn(m.FocusedField()) }

// ToggleFocusedField switches the focused field between following and its own
// value, and reports whether it changed the profile. Following to own, a
// boolean takes the opposite of the value it follows; any other field puts back
// the value it had before, or opens its editor when it has none. Own to
// following remembers the value, so the same key undoes either move.
func (m *DetailModel) ToggleFocusedField() bool {
	if m.Editing() || !m.CanToggleFocusedField() {
		return false
	}
	field := m.FocusedField()
	m.saveError = nil
	if m.isOwn(field) {
		if value, err := profile.ReadField(m.raw, field); err == nil {
			if m.lastOwn == nil {
				m.lastOwn = map[string]string{}
			}
			m.lastOwn[field] = profileEditorValue(value)
		}
		if err := m.raw.Reset(field); err != nil {
			m.saveError = err
			return false
		}
		m.rebuildResolved()
		m.noteChange(field)
		return true
	}
	value, ok := m.ownValueToSet(field)
	if !ok {
		m.BeginEdit()
		return false
	}
	if err := m.setFocusedEditorValue(value); err != nil {
		m.saveError = err
		return false
	}
	m.rebuildResolved()
	m.noteChange(field)
	return true
}

// ownValueToSet picks the value Space gives a field that follows.
func (m DetailModel) ownValueToSet(field string) (string, bool) {
	descriptor, _ := profile.Field(field)
	if descriptor.Kind == profile.PrimitiveBool {
		if current, err := profile.ReadField(m.resolved, field); err == nil {
			if flag, isBool := current.(bool); isBool {
				return strconv.FormatBool(!flag), true
			}
		}
	}
	value, ok := m.lastOwn[field]
	return value, ok
}

// choiceLabels words each choice of a picker. The first choice is the one that
// follows All games, with the value it would give.
func (m DetailModel) choiceLabels(descriptor profile.FieldDescriptor, options []string) []string {
	labels := make([]string, len(options))
	for index, option := range options {
		if option == cycleFollow {
			action := profile.ActionFollowAllGames
			if m.isRoot {
				action = profile.ActionUseSystemDefault
			}
			labels[index] = action + " (" + m.followValueText(descriptor.Key) + ")"
			continue
		}
		labels[index] = choiceWord(descriptor, option)
	}
	return labels
}

// followValueText is the value a field would have if it followed.
func (m DetailModel) followValueText(field string) string {
	descriptor, ok := profile.Field(field)
	if !ok || m.raw == nil {
		return ""
	}
	following := m.raw.Clone()
	if following.Reset(field) != nil {
		return ""
	}
	var explanation profile.FieldExplanation
	var err error
	if m.isRoot {
		explanation, err = following.ExplainAllGamesField(field)
	} else {
		explanation, err = following.ExplainField(field, m.defaults)
	}
	if err != nil {
		return ""
	}
	return profile.ValueText(descriptor, explanation)
}

// choiceWord words one stored value of a field.
func choiceWord(descriptor profile.FieldDescriptor, value string) string {
	switch descriptor.Kind {
	case profile.PrimitiveBool:
		if value == "true" {
			return profile.WordOn
		}
		return profile.WordOff
	case profile.PrimitiveOptionalBool:
		switch value {
		case "true":
			return profile.WordOn
		case "false":
			return profile.WordOff
		}
		return profile.WordNone
	}
	if value == "" {
		return profile.WordNone
	}
	return value
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
// field back to the system default.
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
	m.noteChange(field)
	return true, nil
}

// ResetAll resets every field on the profile. On a game profile only overrides
// are cleared; on the root defaults profile every field returns to the system default.
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
		m.lastChange = "All settings use the system default (" + profile.AllGamesName + ")"
		return true
	}
	if len(m.raw.Overrides) == 0 {
		return false
	}
	m.raw.ResetAll()
	m.rebuildResolved()
	m.lastChange = "All settings follow " + profile.AllGamesName
	return true
}

// RawProfile returns the current working copy. Callers clone it before
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
func (m DetailModel) View() string { return m.ViewFocused(true) }

// ViewFocused omits active controls when another pane owns input.
func (m DetailModel) ViewFocused(focused bool) string {
	s := m.styles
	if s == nil {
		return ""
	}

	if m.Editing() {
		return m.editorView()
	}
	var lines []string
	// add splits multi-line renders so line indices stay exact for scrolling.
	add := func(text string) { lines = append(lines, strings.Split(text, "\n")...) }
	focusedRow := -1
	if len(m.focusableRows) > 0 {
		focusedRow = m.focusableRows[m.cursor]
	}
	if m.editor.Error() != nil {
		add(s.Error.Render("Invalid value: " + m.editor.Error().Error()))
	}

	labelWidth := 0
	for _, row := range m.rows {
		if !row.isHeader() {
			labelWidth = max(labelWidth, ansi.StringWidth(row.label))
		}
	}

	focusedStart, focusedEnd := -1, -1
	for i, row := range m.rows {
		if row.isHeader() {
			if i > 0 {
				add("")
			}
			add(s.Title.MarginBottom(0).Render(row.headerLabel))
			continue
		}

		value, source, overridden := m.fieldText(row.field)
		if i == focusedRow && m.editor.Active() {
			value = m.editor.Value()
		}

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
		body := fmt.Sprintf("%-*s  %-12s  %s", labelWidth, row.label, value, source)
		if m.width > 0 {
			body = ansi.Truncate(body, max(m.width-7, 1), "…")
		}

		if i == focusedRow && focused {
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
			focusedStart = len(lines)
		}
		add("  " + marker + body)
		if i == focusedRow {
			// The effect and description wrap instead of being cut, so the
			// restore sentence is never lost at 80 columns.
			for _, text := range m.focusedNotes(row.field) {
				for _, wrapped := range wrapIndented(text, m.width, "    ") {
					add(s.Dim.Render(wrapped))
				}
			}
			focusedEnd = len(lines) - 1
		}
	}
	if m.height > 0 && len(lines) > m.height && focusedStart >= 0 {
		// Keep the focused row and everything that explains it visible.
		start := max(focusedEnd-m.height+1, 0)
		start = min(start, focusedStart)
		start = min(start, len(lines)-m.height)
		lines = lines[start : start+m.height]
	} else if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

// fieldText returns the value and source words for one field row, from the
// shared vocabulary, and whether the game sets the value itself.
func (m DetailModel) fieldText(field string) (value, source string, own bool) {
	descriptor, ok := profile.Field(field)
	if !ok {
		return "", "", false
	}
	raw := m.raw
	if raw == nil {
		raw = &profile.Profile{}
	}
	var explanation profile.FieldExplanation
	var err error
	if m.isRoot {
		explanation, err = raw.ExplainAllGamesField(field)
	} else {
		explanation, err = raw.ExplainField(field, m.defaults)
	}
	if err != nil {
		return "", "", false
	}
	own = explanation.Origin == profile.OriginThisGame || explanation.Origin == profile.OriginThisGameSame
	return profile.ValueText(descriptor, explanation), profile.SourceText(descriptor, explanation), own
}

// SetOwnValueGames records which games set their own value for each field.
func (m *DetailModel) SetOwnValueGames(games map[string][]string) { m.ownValueGames = games }

// FocusedOwnValueGameCount is the number of games that set their own value for
// the focused field, on the All games page; zero elsewhere.
func (m DetailModel) FocusedOwnValueGameCount() int {
	if !m.isRoot {
		return 0
	}
	return len(m.ownValueGames[m.FocusedField()])
}

// focusedNotes returns the lines of explanation shown under the focused row:
// what the setting does, and on All games where to apply it to the games that
// set their own value.
func (m DetailModel) focusedNotes(field string) []string {
	var notes []string
	if descriptor, ok := profile.Field(field); ok && descriptor.Description != "" {
		notes = append(notes, descriptor.Description)
	}
	if count := len(m.ownValueGames[field]); m.isRoot && count > 0 {
		notes = append(notes, fmt.Sprintf("Actions (0): %s (%s)", profile.ActionApplyToGamesWithOwnValue, profile.GamesText(count)))
	}
	return notes
}

// wrapIndented wraps text to width and indents every line. A width of zero
// means the size is not known yet and the text is returned on one line.
func wrapIndented(text string, width int, indent string) []string {
	if width <= 0 {
		return []string{indent + text}
	}
	wrapped := strings.Split(ansi.Wrap(text, max(width-ansi.StringWidth(indent)-4, 8), ""), "\n")
	for index := range wrapped {
		wrapped[index] = indent + wrapped[index]
	}
	return wrapped
}

// editorHint names the unit and shown range of a numeric field and says how a
// typed value is entered.
func editorHint(descriptor profile.FieldDescriptor) string {
	hint := profile.NumberHint(descriptor)
	if hint == "" {
		return ""
	}
	return hint + " Typing replaces the value."
}

// editorBackspaceHint puts Backspace in the control line of text-like editors.
func editorBackspaceHint(kind EditorValueKind) string {
	if kind == EditorBool || kind == EditorChoice {
		return ""
	}
	return "  Backspace delete"
}

// editorView reserves the local controls before allocating explanatory content.
func (m DetailModel) editorView() string {
	if m.editor.Kind() == EditorChoice && len(m.editor.spec.Labels) > 0 {
		return m.pickerView()
	}
	styles := m.styles
	descriptor := m.focusedDescriptor()
	width := max(m.width, 1)
	lines := []string{
		styles.Title.MarginBottom(0).Render("Edit " + descriptor.Label),
		styles.Dim.Render("Value before edit: " + m.editor.original),
		styles.FocusStyle().Render(m.editor.InputView(width)),
	}
	if hint := editorHint(descriptor); hint != "" {
		lines = append(lines, styles.Dim.Render(hint))
	}
	if m.editor.Error() != nil {
		lines = append(lines, styles.Error.Render("Invalid value: "+m.editor.Error().Error()))
	}
	if m.EditorInputFocused() && (m.editor.Kind() == EditorBool || m.editor.Kind() == EditorChoice) {
		lines = append(lines, styles.Dim.Render("Arrows change value; Space toggles"))
	}
	controls := []string{
		m.editor.Controls(styles),
		styles.Dim.Render("Tab control  Enter " + m.EditorEnterLabel() + editorBackspaceHint(m.editor.Kind()) + "  Esc cancel"),
	}
	if m.height > 0 && len(lines)+len(controls) > m.height {
		lines = append(lines[:1], lines[2:]...)
	}
	lines = append(lines, controls...)
	for index, line := range lines {
		if m.width > 0 {
			lines[index] = ansi.Truncate(line, width, "…")
		}
	}
	return strings.Join(lines, "\n")
}

// pickerView lists the choices of the focused field, windowed around the
// current one, with the controls below.
func (m DetailModel) pickerView() string {
	styles := m.styles
	descriptor := m.focusedDescriptor()
	width := max(m.width, 1)
	labels, cursor := m.editor.ChoiceRows()
	title := styles.Title.MarginBottom(0).Render("Edit " + descriptor.Label)
	if m.editor.search != "" {
		title += styles.Dim.Render("  jump: " + m.editor.search)
	}
	lines := []string{title}
	capacity := len(labels)
	if m.height > 0 {
		capacity = max(m.height-3, 3)
	}
	start, end := visibleRange(cursor, len(labels), capacity)
	for index := start; index < end; index++ {
		row := "  " + labels[index]
		switch {
		case index == cursor && m.editor.InputFocused():
			row = styles.FocusStyle().Render("> " + labels[index])
		case index == cursor:
			row = styles.Selected.Render("> " + labels[index])
		}
		lines = append(lines, row)
	}
	lines = append(lines, m.editor.Controls(styles), styles.Dim.Render("Type to jump  ↑ ↓ choose  Tab control  Enter "+m.EditorEnterLabel()+"  Esc cancel"))
	for index, line := range lines {
		if m.width > 0 {
			lines[index] = ansi.Truncate(line, width, "…")
		}
	}
	return strings.Join(lines, "\n")
}

// cycleFollow is the first choice of a cycling field: clear the value so the
// field follows All games (or the system default on the All games page). It is
// never shown; the row shows the effective value and its source instead.
const cycleFollow = "\x00follow"

func fieldCycleOptions(field string) []string {
	descriptor, ok := profile.Field(field)
	if !ok {
		return nil
	}
	if descriptor.Kind == profile.PrimitiveBool {
		return []string{cycleFollow, "true", "false"}
	}
	if len(descriptor.AllowedValues) == 0 {
		return nil
	}
	options := []string{cycleFollow}
	hasEmpty := false
	for _, value := range descriptor.AllowedValues {
		if value == "" {
			hasEmpty = true
		} else {
			options = append(options, value)
		}
	}
	// An explicit empty value can disable a value inherited from defaults.
	if hasEmpty && field != profile.FieldGPUVRR {
		options = append(options, "")
	}
	return options
}

// rootFieldAtDefault reports whether the All games profile leaves a field at
// the system default.
func (m DetailModel) rootFieldAtDefault(field string) bool {
	if m.raw == nil {
		return true
	}
	explanation, err := m.raw.ExplainAllGamesField(field)
	return err != nil || explanation.Origin == profile.OriginSystemDefault
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
