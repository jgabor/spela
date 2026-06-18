package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/settings"
)

type OptionType int

const (
	OptionTypeBool OptionType = iota
	OptionTypeEnum
	OptionTypePath
	OptionTypeInt
)

type Option struct {
	Key         string
	Label       string
	Description string
	Type        OptionType
	Options     []string
}

type OptionsSection struct {
	Title   string
	Options []Option
}

type OptionsModalModel struct {
	styles         *Styles
	config         *config.Config
	originalConfig *config.Config
	sections       []OptionsSection
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
		sections:  catalogOptionsSections(),
		pathInput: ti,
	}
}

func catalogOptionsSections() []OptionsSection {
	catalog := settings.Catalog()
	sections := make([]OptionsSection, len(catalog))
	for i, section := range catalog {
		opts := make([]Option, len(section.Options))
		for j, opt := range section.Options {
			opts[j] = Option{
				Key:         opt.Key,
				Label:       opt.Label,
				Description: opt.Description,
				Type:        catalogOptionType(opt.Kind),
				Options:     opt.Choices,
			}
		}
		sections[i] = OptionsSection{Title: section.Title, Options: opts}
	}
	return sections
}

func catalogOptionType(kind settings.Kind) OptionType {
	switch kind {
	case settings.KindBool:
		return OptionTypeBool
	case settings.KindPath:
		return OptionTypePath
	case settings.KindInt:
		return OptionTypeInt
	default:
		return OptionTypeEnum
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
			if opt != nil && opt.Type == OptionTypePath {
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

	if opt.Type == OptionTypePath {
		return
	}

	if len(opt.Options) == 0 {
		return
	}

	currentIndex := 0
	for i, o := range opt.Options {
		if o == currentValue {
			currentIndex = i
			break
		}
	}

	newIndex := (currentIndex + direction + len(opt.Options)) % len(opt.Options)
	newValue := opt.Options[newIndex]
	m.setConfigValue(opt.Key, newValue)
	m.modified = true
}

func (m OptionsModalModel) getConfigValue(key string) string {
	if m.config == nil {
		return ""
	}

	switch key {
	case "rescan_on_startup":
		return boolStr(m.config.RescanOnStartup)
	case "auto_update_dlls":
		return boolStr(m.config.AutoUpdateDLLs)
	case "check_updates":
		return boolStr(m.config.CheckUpdates)
	case "steam_path":
		if m.config.SteamPath == "" {
			return "(default)"
		}
		return m.config.SteamPath
	case "dll_cache_path":
		if m.config.DLLCachePath == "" {
			return "(default)"
		}
		return m.config.DLLCachePath
	case "backup_path":
		if m.config.BackupPath == "" {
			return "(default)"
		}
		return m.config.BackupPath
	case "auto_refresh_manifest":
		return boolStr(m.config.AutoRefreshManifest)
	case "manifest_refresh_hours":
		return intStr(m.config.ManifestRefreshHours)
	case "preferred_dll_source":
		if m.config.PreferredDLLSource == "" {
			return "techpowerup"
		}
		return m.config.PreferredDLLSource
	case "show_hints":
		return boolStr(m.config.ShowHints)
	case "compact_mode":
		return boolStr(m.config.CompactMode)
	case "confirm_destructive":
		return boolStr(m.config.ConfirmDestructive)
	case "theme":
		if m.config.Theme == "" {
			return "default"
		}
		return m.config.Theme
	case "log_level":
		return string(m.config.LogLevel)
	}
	return ""
}

func (m *OptionsModalModel) setConfigValue(key, value string) {
	if m.config == nil {
		return
	}

	switch key {
	case "rescan_on_startup":
		m.config.RescanOnStartup = value == "true"
	case "auto_update_dlls":
		m.config.AutoUpdateDLLs = value == "true"
	case "check_updates":
		m.config.CheckUpdates = value == "true"
	case "steam_path":
		if value == "(default)" {
			m.config.SteamPath = ""
		} else {
			m.config.SteamPath = value
		}
	case "dll_cache_path":
		if value == "(default)" {
			m.config.DLLCachePath = ""
		} else {
			m.config.DLLCachePath = value
		}
	case "backup_path":
		if value == "(default)" {
			m.config.BackupPath = ""
		} else {
			m.config.BackupPath = value
		}
	case "auto_refresh_manifest":
		m.config.AutoRefreshManifest = value == "true"
	case "manifest_refresh_hours":
		var v int
		_, _ = fmt.Sscanf(value, "%d", &v)
		m.config.ManifestRefreshHours = v
	case "preferred_dll_source":
		m.config.PreferredDLLSource = value
	case "show_hints":
		m.config.ShowHints = value == "true"
		m.styles.SetShowHints(m.config.ShowHints)
	case "compact_mode":
		m.config.CompactMode = value == "true"
	case "confirm_destructive":
		m.config.ConfirmDestructive = value == "true"
	case "theme":
		m.config.Theme = value
	case "log_level":
		m.config.LogLevel = config.LogLevel(value)
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

	sections := m.sections
	if m.embedded && m.sectionCursor < len(m.sections) {
		sections = []OptionsSection{m.sections[m.sectionCursor]}
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

			label := fmt.Sprintf("%s%-22s: ", cursor, opt.Label)
			b.WriteString(style.Render(label))

			if isCurrentOption && m.editingPath && opt.Type == OptionTypePath {
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
	if currentOption != nil {
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
	} else if currentOption != nil && currentOption.Type == OptionTypePath {
		hint = "\n↑↓:navigate • enter:edit • s:save • esc:close"
	} else {
		hint = "\n↑↓:navigate • ←→:change • s:save • esc:close"
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

func (m OptionsModalModel) getCurrentOption() *Option {
	if m.sectionCursor >= len(m.sections) {
		return nil
	}
	section := m.sections[m.sectionCursor]
	if m.optionCursor >= len(section.Options) {
		return nil
	}
	return &section.Options[m.optionCursor]
}
