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
	config        *config.Config
	sections      []config.Section
	sectionCursor int
	optionCursor  int
	modified      bool
	editingPath   bool
	pathInput     textinput.Model
	width         int
}

func (m *OptionsModalModel) SetSize(width, _ int) { m.width = width }

type optionsSavedMsg struct {
	config *config.Config
}

type optionsSaveErrorMsg struct {
	err error
}

func NewOptionsModal(styles *Styles) OptionsModalModel {
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.SetWidth(40)

	return OptionsModalModel{
		styles:    styles,
		sections:  config.Sections(config.VisibilityTUI),
		pathInput: ti,
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
	m.config = cfg
	m.modified = false
	m.editingPath = false
}

func (m OptionsModalModel) Update(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
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
	m.editingPath = true
}

func (m OptionsModalModel) updatePathEditing(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			opt := m.getCurrentOption()
			if opt != nil {
				value := m.pathInput.Value()
				if value == "" {
					value = "(default)"
				}
				m.setConfigValue(opt.Key, value)
				m.modified = true
			}
			m.editingPath = false
			m.pathInput.Blur()
			return m, nil
		case "esc":
			m.editingPath = false
			m.pathInput.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m *OptionsModalModel) moveCursor(direction int) {
	section := m.sections[m.sectionCursor]
	if len(section.Options) == 0 {
		return
	}
	m.optionCursor = (m.optionCursor + direction + len(section.Options)) % len(section.Options)
}

func (m *OptionsModalModel) cycleValue(direction int) {
	opt := m.getCurrentOption()
	if opt == nil || opt.Kind == config.KindPath || len(opt.Choices) == 0 {
		return
	}
	currentIndex := max(slices.Index(opt.Choices, m.getConfigValue(opt.Key)), 0)
	newIndex := (currentIndex + direction + len(opt.Choices)) % len(opt.Choices)
	m.setConfigValue(opt.Key, opt.Choices[newIndex])
	m.modified = true
}

func (m OptionsModalModel) getConfigValue(key string) string {
	if m.config == nil {
		return ""
	}
	option := config.OptionByKey(key)
	if option == nil || !option.Visibility.Includes(config.VisibilityTUI) {
		return ""
	}
	value := option.Get(m.config)
	if value == "" && option.Kind == config.KindPath {
		return "(default)"
	}
	if value == "" && option.Kind == config.KindEnum {
		return option.Get(config.Default())
	}
	return value
}

func (m *OptionsModalModel) setConfigValue(key, value string) {
	if m.config == nil {
		return
	}
	option := config.OptionByKey(key)
	if option == nil || !option.Visibility.Includes(config.VisibilityTUI) {
		return
	}
	if value == "(default)" && option.Kind == config.KindPath {
		value = ""
	}
	if option.Set(m.config, value) != nil {
		return
	}
	if key == "show_hints" {
		m.styles.SetShowHints(m.config.ShowHints)
	}
}

func (m OptionsModalModel) save() (OptionsModalModel, tea.Cmd) {
	cfg := m.config
	m.modified = false
	return m, func() tea.Msg {
		if err := cfg.Save(); err != nil {
			return optionsSaveErrorMsg{err: err}
		}
		return optionsSavedMsg{config: cfg}
	}
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
