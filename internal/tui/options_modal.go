package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

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
	saveError     error
}

func (m *OptionsModalModel) SetSize(width, _ int) { m.width = width }

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
	m.editingPath = false
}

func (m OptionsModalModel) Update(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	if m.saving {
		return m, nil
	}
	if m.editingPath {
		return m.updatePathEditing(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "left", "h":
			m.cycleValue(-1)
		case "right", "l":
			m.cycleValue(1)
		case "enter":
			opt := m.getCurrentOption()
			if opt != nil && opt.Kind == config.KindPath {
				m.startPathEditing()
				return m, nil
			}
		case "s":
			return m.save()
		}
	}

	return m, nil
}

func (m *OptionsModalModel) startPathEditing() {
	opt := m.getCurrentOption()
	if opt == nil {
		return
	}

	currentValue := m.getConfigValue(opt.Key)
	if currentValue == "(default)" {
		currentValue = ""
	}

	m.pathInput.SetValue(currentValue)
	m.pathInput.Focus()
	m.editor.Begin(EditorSpec{Key: opt.Key, Kind: EditorPath}, currentValue)
	m.editingPath = true
}

func (m OptionsModalModel) updatePathEditing(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			opt := m.getCurrentOption()
			if opt != nil {
				m.editor.Set(m.pathInput.Value())
				value, err := m.editor.Commit()
				if err != nil {
					return m, nil
				}
				if value == "" {
					value = "(default)"
				}
				m.setConfigValue(opt.Key, value)
			}
			m.editingPath = false
			m.pathInput.Blur()
			return m, nil
		case "esc":
			m.editor.Cancel()
			m.editingPath = false
			m.pathInput.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	m.editor.Set(m.pathInput.Value())
	return m, cmd
}

func (m *OptionsModalModel) moveCursor(direction int) {
	section := m.sections[m.sectionCursor]
	if len(section.Options) == 0 {
		return
	}
	m.optionCursor = (m.optionCursor + direction + len(section.Options)) % len(section.Options)
}

func (m *OptionsModalModel) UpdateList(key tea.KeyPressMsg) bool {
	switch key.String() {
	case "up", "k":
		m.moveCursor(-1)
		return true
	case "down", "j":
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
	builder.WriteString(m.styles.Dim.Render("←/→ group"))
	builder.WriteString("\n\n")
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

func (m OptionsModalModel) DetailView() string {
	option := m.getCurrentOption()
	if option == nil {
		return m.styles.Dim.Render("No setting selected")
	}
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render(option.Label))
	builder.WriteString("\n\n")
	builder.WriteString(option.Description)
	builder.WriteString("\n\n")
	builder.WriteString(m.styles.Dim.Render("Saved value"))
	builder.WriteString("\n")
	builder.WriteString(m.styles.DLSS.Render(option.Get(m.config)))
	builder.WriteString("\n\n")
	builder.WriteString(m.styles.Dim.Render("Draft value"))
	builder.WriteString("\n")
	draft := m.getConfigValue(option.Key)
	if m.editingPath {
		draft = m.pathInput.View()
	}
	builder.WriteString(m.styles.DLSS.Render(draft))
	builder.WriteString("\n\n")
	if m.editingPath {
		builder.WriteString(m.styles.Selected.Render("Enter:commit  Esc:cancel"))
	} else if m.saving {
		builder.WriteString(m.styles.Dim.Render("Saving…"))
	} else if m.saveError != nil {
		builder.WriteString(m.styles.Error.Render("Save failed: " + m.saveError.Error()))
		builder.WriteString("\n")
		builder.WriteString(m.styles.Selected.Render("Draft retained  s:retry  Esc:cancel"))
	} else if m.modified {
		builder.WriteString(m.styles.Selected.Render("Unsaved changes  s:save"))
	} else {
		builder.WriteString(m.styles.Dim.Render("Enter:edit  ←/→:change"))
	}
	return builder.String()
}

func (m *OptionsModalModel) cycleValue(direction int) {
	opt := m.getCurrentOption()
	if opt == nil || opt.Kind == config.KindPath || len(opt.Choices) == 0 {
		return
	}
	current := m.getConfigValue(opt.Key)
	currentIndex := max(slices.Index(opt.Choices, current), 0)
	newIndex := (currentIndex + direction + len(opt.Choices)) % len(opt.Choices)
	m.editor.Begin(EditorSpec{Key: opt.Key, Kind: EditorChoice, Choices: append([]string(nil), opt.Choices...)}, current)
	m.editor.Set(opt.Choices[newIndex])
	value, err := m.editor.Commit()
	if err != nil {
		return
	}
	m.setConfigValue(opt.Key, value)
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
	if m.draft == nil || !m.modified {
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
	if m.config != nil {
		m.draft = m.config.Clone()
	}
	m.modified = false
	m.editingPath = false
	m.pathInput.Blur()
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

	if m.sectionCursor >= len(m.sections) {
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

	var hint string
	if m.editingPath {
		hint = "\nenter:confirm • esc:cancel"
	} else if currentOption != nil && currentOption.Kind == config.KindPath {
		hint = "\n↑↓:navigate • enter:edit • s:save • esc:close"
	} else {
		hint = "\n↑↓:navigate • ←→:change • s:save • esc:close"
	}
	if compactLabels {
		hint = "\n↑↓ • ←→ • s:save"
	}
	if h := s.RenderHint(hint); h != "" {
		b.WriteString(h)
	}

	return b.String()
}

func (m OptionsModalModel) getCurrentOption() *config.Option {
	if m.sectionCursor >= len(m.sections) {
		return nil
	}
	section := m.sections[m.sectionCursor]
	if m.optionCursor >= len(section.Options) {
		return nil
	}
	return &section.Options[m.optionCursor]
}
