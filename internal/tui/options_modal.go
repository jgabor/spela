package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
)

type OptionsModalModel struct {
	styles        *Styles
	config        *config.Config // last successfully persisted/live value
	draft         *config.Config
	saveConfig    func(*config.Config) error
	sections      []config.Section
	sectionCursor int
	optionCursor  int
	modified      bool
	saving        bool
	editingPath   bool
	pathInput     textinput.Model
	editor        EditorHost
	width         int
	height        int
	saveError     error
}

func (m *OptionsModalModel) SetSize(width, height int) {
	m.width, m.height = width, height
	m.pathInput.SetWidth(max(width-6, 1))
}

type optionsSavedMsg struct{ config *config.Config }

type optionsSaveErrorMsg struct {
	err error
}

func NewOptionsModal(styles *Styles) OptionsModalModel {
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.SetWidth(40)

	return OptionsModalModel{
		styles:     styles,
		sections:   config.Sections(config.VisibilityTUI),
		pathInput:  ti,
		saveConfig: func(configuration *config.Config) error { return configuration.Save() },
	}
}

func (m *OptionsModalModel) SetSaveConfig(save func(*config.Config) error) {
	if save != nil {
		m.saveConfig = save
	}
}

// SyncNavSection aligns the embedded settings view with navigation state.
func (m *OptionsModalModel) SyncNavSection(section nav.SettingsSection) {
	if int(section) < 0 || int(section) >= len(m.sections) {
		return
	}
	m.sectionCursor = int(section)
	if m.optionCursor >= len(m.sections[m.sectionCursor].Options) {
		m.optionCursor = 0
	}
}

// OpenEmbedded activates settings as a full destination (not a modal overlay).
func (m *OptionsModalModel) OpenEmbedded(cfg *config.Config) {
	if cfg == nil {
		cfg = config.Default()
	}
	if m.config == nil || (!m.modified && !m.saving) {
		m.config = cfg.Clone()
		m.draft = cfg.Clone()
		m.modified = false
		m.saveError = nil
	}
}

func (m OptionsModalModel) Editing() bool                    { return m.editor.Active() }
func (m OptionsModalModel) Dirty() bool                      { return m.modified }
func (m OptionsModalModel) Busy() bool                       { return m.saving }
func (m OptionsModalModel) SaveError() error                 { return m.saveError }
func (m OptionsModalModel) EditorInputFocused() bool         { return m.editor.InputFocused() }
func (m OptionsModalModel) EditorInputKind() EditorValueKind { return m.editor.Kind() }
func (m OptionsModalModel) EditorEnterLabel() string         { return m.editor.EnterLabel() }

// Update keeps standalone basic-key input working. Shell command routing uses
// UpdateAction after resolving the active pane and input mode.
func (m OptionsModalModel) Update(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	if m.saving {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m.UpdateEditorInput(msg)
	}
	var action KeyAction
	if m.Editing() {
		switch key.String() {
		case "tab":
			action = ActionFocusNext
		case "enter":
			action = ActionEditCommit
		case "ctrl+s":
			action = ActionEditSave
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
			return m.UpdateEditorInput(msg)
		}
	} else {
		switch key.String() {
		case "up":
			action = ActionDetailPreviousItem
		case "down":
			action = ActionDetailNextItem
		case "enter":
			action = ActionDetailConfirm
		case "space":
			action = ActionDetailCycle
		case "ctrl+s":
			action = ActionDetailSave
		default:
			return m, nil
		}
	}
	return m.UpdateAction(action)
}

// UpdateAction applies semantic commands; Browse arrows never mutate a value.
func (m OptionsModalModel) UpdateAction(action KeyAction) (OptionsModalModel, tea.Cmd) {
	if m.saving {
		return m, nil
	}
	if m.Editing() {
		switch action {
		case ActionFocusNext:
			m.editor.FocusNext()
			m.syncInputFocus()
		case ActionEditCancel:
			m.cancelEditor()
		case ActionEditCommit:
			if m.editor.focus == EditorFocusCancel {
				m.cancelEditor()
				return m, nil
			}
			save := m.editor.focus == EditorFocusSave
			if m.applyEditor() && save {
				return m.save()
			}
		case ActionEditSave:
			if m.applyEditor() {
				return m.save()
			}
		case ActionEditDelete:
			return m.UpdateEditorInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
		case ActionDetailDecrease, ActionDetailIncrease:
			direction, code := 1, tea.KeyRight
			if action == ActionDetailDecrease {
				direction, code = -1, tea.KeyLeft
			}
			if m.EditorInputFocused() && m.editingPath {
				return m.UpdateEditorInput(tea.KeyPressMsg{Code: code})
			}
			m.editor.Move(direction)
		case ActionDetailPreviousItem:
			if m.EditorInputFocused() {
				m.editor.Cycle(-1)
			}
		case ActionDetailNextItem:
			if m.EditorInputFocused() {
				m.editor.Cycle(1)
			}
		case ActionEditToggle:
			if m.EditorInputFocused() && m.editingPath {
				return m.UpdateEditorInput(tea.KeyPressMsg{Code: ' ', Text: " "})
			}
			m.editor.Toggle()
		}
		return m, nil
	}
	switch action {
	case ActionDetailPreviousItem, ActionListPrevious:
		m.moveCursor(-1)
	case ActionDetailNextItem, ActionListNext:
		m.moveCursor(1)
	case ActionDetailConfirm:
		if !m.CanCycleFocusedField() {
			m.beginEditor()
		}
	case ActionDetailCycle:
		m.cycleValue(1)
	case ActionDetailSave:
		return m.save()
	case ActionCancelDraft:
		m.CancelDraft()
	}
	return m, nil
}

func (m *OptionsModalModel) beginEditor() {
	option := m.getCurrentOption()
	if option == nil || m.draft == nil || m.saving {
		return
	}
	value := option.Get(m.draft)
	kind := EditorChoice
	switch option.Kind {
	case config.KindBool:
		kind = EditorBool
	case config.KindPath:
		kind = EditorPath
	case config.KindInt:
		kind = EditorInteger
	}
	m.editor.Begin(EditorSpec{
		Key: option.Key, Kind: kind, Choices: append([]string(nil), option.Choices...),
		Validate: func(value string) error { return option.Set(m.draft.Clone(), value) },
	}, value)
	m.saveError = nil
	m.editingPath = option.Kind == config.KindPath
	if m.editingPath {
		m.pathInput.SetValue(value)
		m.pathInput.CursorEnd()
		m.pathInput.Focus()
	}
}

func (m *OptionsModalModel) startPathEditing() { m.beginEditor() }

func (m *OptionsModalModel) syncInputFocus() {
	if m.editingPath && m.EditorInputFocused() {
		m.pathInput.Focus()
	} else {
		m.pathInput.Blur()
	}
}

func (m *OptionsModalModel) cancelEditor() {
	m.editor.Cancel()
	m.editingPath = false
	m.pathInput.Blur()
}

func (m *OptionsModalModel) applyEditor() bool {
	if m.editingPath {
		m.editor.Set(m.pathInput.Value())
	}
	value, err := m.editor.Commit()
	if err != nil {
		m.syncInputFocus()
		return false
	}
	option := m.getCurrentOption()
	if option == nil {
		return false
	}
	if err := option.Set(m.draft, value); err != nil {
		m.editor.active = true
		m.editor.focus = EditorFocusInput
		m.editor.err = err
		m.syncInputFocus()
		return false
	}
	m.modified = m.config == nil || !configsEqual(m.config, m.draft)
	m.saveError = nil
	m.editingPath = false
	m.pathInput.Blur()
	return true
}

// UpdateEditorInput accepts text and cursor operations only in the active input.
func (m OptionsModalModel) UpdateEditorInput(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	if m.saving || !m.EditorInputFocused() {
		return m, nil
	}
	if m.editingPath {
		var command tea.Cmd
		m.pathInput, command = m.pathInput.Update(msg)
		m.editor.Set(m.pathInput.Value())
		return m, command
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		m.editor.UpdateInput(key)
	}
	return m, nil
}

func (m OptionsModalModel) updatePathEditing(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	return m.Update(msg)
}

func (m *OptionsModalModel) moveCursor(direction int) {
	if m.sectionCursor < 0 || m.sectionCursor >= len(m.sections) {
		return
	}
	section := m.sections[m.sectionCursor]
	if len(section.Options) == 0 {
		return
	}
	m.optionCursor = (m.optionCursor + direction + len(section.Options)) % len(section.Options)
}

func (m *OptionsModalModel) UpdateList(key tea.KeyPressMsg) bool {
	switch key.String() {
	case "up":
		m.moveCursor(-1)
		return true
	case "down":
		m.moveCursor(1)
		return true
	}
	return false
}

func (m OptionsModalModel) ListView(focused bool) string {
	if m.sectionCursor < 0 || m.sectionCursor >= len(m.sections) {
		return m.styles.Dim.Render("No settings")
	}
	section := m.sections[m.sectionCursor]
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render(section.Title))
	builder.WriteString("\n")
	if focused && !m.Editing() && !m.saving {
		builder.WriteString(m.styles.Dim.Render("←/→ group"))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	for index, option := range section.Options {
		style := m.styles.Dim
		prefix := "  "
		if index == m.optionCursor {
			prefix = "> "
			if focused {
				style = m.styles.FocusStyle()
			} else {
				style = m.styles.Selected
			}
		}
		builder.WriteString(style.Render(prefix + option.Label))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (m OptionsModalModel) DetailView() string { return m.DetailViewFocused(true) }

func (m OptionsModalModel) DetailViewFocused(focused bool) string {
	option := m.getCurrentOption()
	if option == nil {
		return m.styles.Dim.Render("No setting selected")
	}
	if m.Editing() {
		return m.editorView()
	}
	styles := m.styles
	lines := []string{
		styles.Title.Render(option.Label),
		styles.Dim.Render("Saved value"),
		styles.DLSS.Render(option.Get(m.config)),
		styles.Dim.Render("Draft value"),
		styles.DLSS.Render(m.getConfigValue(option.Key)),
	}
	switch {
	case m.saving:
		lines = append(lines, styles.Dim.Render("Saving…"))
	case m.saveError != nil:
		lines = append(lines, styles.Error.Render("Save failed: "+m.saveError.Error()), styles.Selected.Render("Draft retained"))
	case m.modified:
		lines = append(lines, styles.Warning.Render("Unsaved changes"))
	}
	// Descriptions are supplementary. Values and operation results come first.
	if m.height <= 0 || len(lines)+2 <= m.height {
		lines = append(lines, "", styles.Dim.Render(option.Description))
	}
	for index, line := range lines {
		if m.width > 0 {
			lines[index] = ansi.Truncate(line, max(m.width-4, 1), "…")
		}
	}
	return strings.Join(lines, "\n")
}

func (m OptionsModalModel) editorView() string {
	styles := m.styles
	option := m.getCurrentOption()
	if option == nil {
		return styles.Dim.Render("No setting selected")
	}
	value := m.editor.InputView(max(m.width-4, 1))
	if m.editingPath {
		value = m.pathInput.View()
	}
	lines := []string{
		styles.Title.Render("Edit " + option.Label),
		styles.Dim.Render("Value before edit: " + m.editor.original),
		value,
	}
	if m.editor.Error() != nil {
		lines = append(lines, styles.Error.Render("Invalid value: "+m.editor.Error().Error()))
	}
	if m.EditorInputFocused() && (m.editor.Kind() == EditorBool || m.editor.Kind() == EditorChoice) {
		lines = append(lines, styles.Dim.Render("Arrows change value; Space toggles"))
	}
	controls := []string{
		m.editor.Controls(styles),
		styles.Dim.Render("Tab control  Enter " + m.EditorEnterLabel() + "  Ctrl+S save"),
	}
	if m.height > 0 && len(lines)+len(controls) > m.height {
		// Keep input and validation errors ahead of the original-value summary.
		lines = append(lines[:1], lines[2:]...)
	}
	lines = append(lines, controls...)
	for index, line := range lines {
		if m.width > 0 {
			lines[index] = ansi.Truncate(line, max(m.width-4, 1), "…")
		}
	}
	return strings.Join(lines, "\n")
}

// CanCycleFocusedField reports whether Space can change the selected setting.
func (m OptionsModalModel) CanCycleFocusedField() bool {
	option := m.getCurrentOption()
	return option != nil && m.draft != nil && option.Kind != config.KindPath && len(option.Choices) > 0 && !m.saving
}

// cycleValue changes only the draft and leaves focus on the setting.
func (m *OptionsModalModel) cycleValue(direction int) {
	if m.Editing() || !m.CanCycleFocusedField() {
		return
	}
	option := m.getCurrentOption()
	current := option.Get(m.draft)
	index := 0
	for choiceIndex, choice := range option.Choices {
		if choice == current {
			index = choiceIndex
			break
		}
	}
	step := 1
	if direction < 0 {
		step = -1
	}
	value := option.Choices[(index+step+len(option.Choices))%len(option.Choices)]
	m.saveError = option.Set(m.draft, value)
	m.modified = m.config == nil || !configsEqual(m.config, m.draft)
}

func (m OptionsModalModel) getConfigValue(key string) string {
	if m.draft == nil {
		return ""
	}
	option := config.OptionByKey(key)
	if option == nil || !option.Visibility.Includes(config.VisibilityTUI) {
		return ""
	}
	value := option.Get(m.draft)
	if value == "" && option.Kind == config.KindPath {
		return "(default)"
	}
	if value == "" && option.Kind == config.KindEnum {
		return option.Get(config.Default())
	}
	return value
}

func (m *OptionsModalModel) setConfigValue(key, value string) {
	if m.draft == nil {
		return
	}
	option := config.OptionByKey(key)
	if option == nil || !option.Visibility.Includes(config.VisibilityTUI) {
		return
	}
	if value == "(default)" && option.Kind == config.KindPath {
		value = ""
	}
	if option.Set(m.draft, value) != nil {
		return
	}
	m.modified = m.config == nil || !configsEqual(m.config, m.draft)
	m.saveError = nil
}

func (m OptionsModalModel) save() (OptionsModalModel, tea.Cmd) {
	if m.saving || m.draft == nil || !m.modified {
		return m, nil
	}
	cfg := m.draft.Clone()
	saveConfig := m.saveConfig
	m.saving = true
	m.saveError = nil
	return m, func() tea.Msg {
		if err := saveConfig(cfg); err != nil {
			return optionsSaveErrorMsg{err: err}
		}
		return optionsSavedMsg{config: cfg}
	}
}

func (m *OptionsModalModel) CancelDraft() {
	if m.saving {
		return
	}
	if m.config != nil {
		m.draft = m.config.Clone()
	}
	m.modified = false
	m.cancelEditor()
	m.saveError = nil
}

func (m *OptionsModalModel) CompleteSave(configuration *config.Config) {
	if configuration == nil {
		return
	}
	m.config = configuration.Clone()
	m.draft = configuration.Clone()
	m.modified = false
	m.saving = false
	m.saveError = nil
}

func configsEqual(left, right *config.Config) bool {
	if left == nil || right == nil {
		return left == right
	}
	for _, option := range config.Options() {
		if !option.Visibility.Includes(config.VisibilityTUI) {
			continue
		}
		if option.Get(left) != option.Get(right) {
			return false
		}
	}
	return true
}

func (m *OptionsModalModel) renderOptionsBody() string {
	s := m.styles
	var b strings.Builder

	labelWidth := 22
	compactLabels := m.width < 60

	if m.sectionCursor < 0 || m.sectionCursor >= len(m.sections) {
		return ""
	}
	section := m.sections[m.sectionCursor]
	for optIndex, opt := range section.Options {
		cursor := "  "
		style := s.Normal
		valueStyle := s.DLSS

		isCurrentOption := optIndex == m.optionCursor

		if isCurrentOption {
			cursor = "> "
			style = s.Selected
		}

		optionLabel := opt.Label
		label := fmt.Sprintf("%s%-*s: ", cursor, labelWidth, optionLabel)
		if compactLabels {
			labelRunes := []rune(optionLabel)
			if len(labelRunes) > 14 {
				optionLabel = string(labelRunes[:13]) + "…"
			}
			label = fmt.Sprintf("%s%s: ", cursor, optionLabel)
		}
		b.WriteString(style.Render(label))

		if isCurrentOption && m.editingPath && opt.Kind == config.KindPath {
			b.WriteString(m.pathInput.View())
		} else {
			value := m.getConfigValue(opt.Key)
			b.WriteString(valueStyle.Render(value))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	currentOption := m.getCurrentOption()
	if currentOption != nil && !compactLabels {
		b.WriteString(s.Dim.Render(currentOption.Description))
		b.WriteString("\n")
	}

	if m.modified {
		b.WriteString(s.Warning.Render("(modified)"))
		b.WriteString("\n")
	}

	if m.Editing() {
		b.WriteString(m.editor.Controls(s))
		b.WriteString("\n")
		b.WriteString(s.Dim.Render("Tab control  Enter " + m.EditorEnterLabel() + "  Ctrl+S save"))
	}

	return b.String()
}

func (m OptionsModalModel) getCurrentOption() *config.Option {
	if m.sectionCursor < 0 || m.sectionCursor >= len(m.sections) {
		return nil
	}
	section := m.sections[m.sectionCursor]
	if m.optionCursor < 0 || m.optionCursor >= len(section.Options) {
		return nil
	}
	return &section.Options[m.optionCursor]
}
