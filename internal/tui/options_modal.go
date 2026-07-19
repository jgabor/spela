package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
)

type OptionsModalModel struct {
	styles         *Styles
	config         *config.Config
	originalConfig *config.Config
	sections       []config.Section
	sectionCursor  int
	optionCursor   int
	modified       bool
	visible        bool
	embedded       bool // true when rendered as Settings destination (not a modal)
	editingPath    bool
	pathInput      textinput.Model
	width          int
	height         int
}

// Compile-time check that OptionsModalModel implements Dialog.
var _ Dialog = (*OptionsModalModel)(nil)

type optionsSavedMsg struct {
	config *config.Config
}

type optionsCancelledMsg struct{}

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

func (m *OptionsModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *OptionsModalModel) Open(cfg *config.Config) {
	m.embedded = false
	m.visible = true
	m.config = cfg
	m.originalConfig = cfg.Clone()
	m.sectionCursor = 0
	m.optionCursor = 0
	m.modified = false
	m.editingPath = false
}

// OpenEmbedded activates settings as a full destination (not a modal overlay).
func (m *OptionsModalModel) OpenEmbedded(cfg *config.Config) {
	m.embedded = true
	m.visible = true
	m.config = cfg
	m.originalConfig = cfg.Clone()
	m.sectionCursor = 0
	m.optionCursor = 0
	m.modified = false
	m.editingPath = false
}

func (m *OptionsModalModel) Close() {
	m.visible = false
}

func (m OptionsModalModel) Visible() bool {
	return m.visible
}

// Update implements Dialog.
func (m *OptionsModalModel) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	return m.updateOptions(msg)
}

func (m *OptionsModalModel) updateOptions(msg tea.Msg) (Dialog, tea.Cmd) {
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
		case "esc":
			if m.embedded {
				return m, nil
			}
			m.visible = false
			m.config = m.originalConfig
			return m, func() tea.Msg {
				return optionsCancelledMsg{}
			}
		case "q":
			if m.embedded {
				return m, nil
			}
			m.visible = false
			m.config = m.originalConfig
			return m, func() tea.Msg {
				return optionsCancelledMsg{}
			}
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

func (m *OptionsModalModel) updatePathEditing(msg tea.Msg) (Dialog, tea.Cmd) {
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
	if m.embedded {
		section := m.sections[m.sectionCursor]
		if len(section.Options) == 0 {
			return
		}
		m.optionCursor = (m.optionCursor + direction + len(section.Options)) % len(section.Options)
		return
	}

	totalOptions := m.totalOptions()
	if totalOptions == 0 {
		return
	}

	flatIndex := m.flatIndex()
	flatIndex = (flatIndex + direction + totalOptions) % totalOptions
	m.setFromFlatIndex(flatIndex)
}

func (m OptionsModalModel) totalOptions() int {
	count := 0
	for _, section := range m.sections {
		count += len(section.Options)
	}
	return count
}

func (m OptionsModalModel) flatIndex() int {
	index := 0
	for i := range m.sectionCursor {
		index += len(m.sections[i].Options)
	}
	return index + m.optionCursor
}

func (m *OptionsModalModel) setFromFlatIndex(flatIndex int) {
	for i, section := range m.sections {
		if flatIndex < len(section.Options) {
			m.sectionCursor = i
			m.optionCursor = flatIndex
			return
		}
		flatIndex -= len(section.Options)
	}
}

func (m *OptionsModalModel) cycleValue(direction int) {
	if m.sectionCursor >= len(m.sections) {
		return
	}
	section := m.sections[m.sectionCursor]
	if m.optionCursor >= len(section.Options) {
		return
	}
	opt := section.Options[m.optionCursor]

	currentValue := m.getConfigValue(opt.Key)

	if opt.Kind == config.KindPath {
		return
	}

	if len(opt.Choices) == 0 {
		return
	}

	currentIndex := 0
	for i, o := range opt.Choices {
		if o == currentValue {
			currentIndex = i
			break
		}
	}

	newIndex := (currentIndex + direction + len(opt.Choices)) % len(opt.Choices)
	newValue := opt.Choices[newIndex]
	m.setConfigValue(opt.Key, newValue)
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

func (m *OptionsModalModel) save() (Dialog, tea.Cmd) {
	cfg := m.config
	m.visible = false
	m.modified = false
	return m, func() tea.Msg {
		if err := cfg.Save(); err != nil {
			return optionsSaveErrorMsg{err: err}
		}
		return optionsSavedMsg{config: cfg}
	}
}

// ViewInline renders options content without modal positioning (Settings destination).
func (m *OptionsModalModel) ViewInline() string {
	if !m.visible {
		return ""
	}
	return m.renderOptionsBody()
}

// View implements Dialog.
func (m *OptionsModalModel) View() string {
	if !m.visible {
		return ""
	}

	s := m.styles
	t := s.Theme

	modalWidth := 54
	modalHeight := m.calculateModalHeight()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(modalWidth).
		Padding(1, 2)

	modal := boxStyle.Render(m.renderOptionsBody())

	centerX := (m.width - modalWidth - 8) / 2
	centerY := (m.height - modalHeight - 8) / 2
	if centerX < 0 {
		centerX = 0
	}
	if centerY < 0 {
		centerY = 0
	}

	positionedStyle := lipgloss.NewStyle().
		MarginLeft(centerX).
		MarginTop(centerY)

	return positionedStyle.Render(modal)
}

func (m *OptionsModalModel) renderOptionsBody() string {
	s := m.styles
	var b strings.Builder

	flatIndex := 0
	currentFlat := m.flatIndex()
	labelWidth := 22
	compactLabels := m.width < 60

	sections := m.sections
	if m.embedded && m.sectionCursor < len(m.sections) {
		sections = []config.Section{m.sections[m.sectionCursor]}
	}

	for _, section := range sections {
		if !m.embedded {
			b.WriteString(s.Dim.Render(section.Title))
			b.WriteString("\n")
		}

		for optIndex, opt := range section.Options {
			cursor := "  "
			style := s.Normal
			valueStyle := s.DLSS

			isCurrentOption := flatIndex == currentFlat
			if m.embedded {
				isCurrentOption = optIndex == m.optionCursor
			}

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
			flatIndex++
		}
		b.WriteString("\n")
	}

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

func (m OptionsModalModel) calculateModalHeight() int {
	height := 4
	for _, section := range m.sections {
		height += 1 + len(section.Options) + 1
	}
	height += 4
	return height
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
