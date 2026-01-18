package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/config"
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
	config         *config.Config
	originalConfig *config.Config
	sections       []OptionsSection
	sectionCursor  int
	optionCursor   int
	modified       bool
	visible        bool
	width          int
	height         int
}

type optionsSavedMsg struct {
	config *config.Config
}

type optionsCancelledMsg struct{}

func NewOptionsModal() OptionsModalModel {
	return OptionsModalModel{
		sections: buildOptionsSections(),
	}
}

func buildOptionsSections() []OptionsSection {
	return []OptionsSection{
		{
			Title: "Startup",
			Options: []Option{
				{
					Key:         "rescan_on_startup",
					Label:       "Re-scan on startup",
					Description: "Scan for new games when spela starts",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
				{
					Key:         "auto_update_dlls",
					Label:       "Auto-update DLLs",
					Description: "Automatically update DLLs on launch",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
				{
					Key:         "check_updates",
					Label:       "Check for updates",
					Description: "Check for spela updates on startup",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
			},
		},
		{
			Title: "Paths",
			Options: []Option{
				{
					Key:         "steam_path",
					Label:       "Steam path",
					Description: "Custom Steam installation path",
					Type:        OptionTypePath,
				},
				{
					Key:         "dll_cache_path",
					Label:       "DLL cache",
					Description: "Path to store downloaded DLLs",
					Type:        OptionTypePath,
				},
				{
					Key:         "backup_path",
					Label:       "Backup path",
					Description: "Path for game save backups",
					Type:        OptionTypePath,
				},
			},
		},
		{
			Title: "DLL management",
			Options: []Option{
				{
					Key:         "auto_refresh_manifest",
					Label:       "Auto-refresh manifest",
					Description: "Automatically refresh DLL manifest",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
				{
					Key:         "manifest_refresh_hours",
					Label:       "Refresh interval",
					Description: "Hours between manifest refreshes",
					Type:        OptionTypeInt,
					Options:     []string{"1", "6", "12", "24", "48", "168"},
				},
				{
					Key:         "preferred_dll_source",
					Label:       "DLL source",
					Description: "Preferred source for DLL downloads",
					Type:        OptionTypeEnum,
					Options:     []string{"techpowerup", "github"},
				},
			},
		},
		{
			Title: "Display",
			Options: []Option{
				{
					Key:         "show_hints",
					Label:       "Show hints",
					Description: "Display keyboard shortcut hints",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
				{
					Key:         "theme",
					Label:       "Theme",
					Description: "Color theme for the interface",
					Type:        OptionTypeEnum,
					Options:     []string{"default", "dark"},
				},
				{
					Key:         "compact_mode",
					Label:       "Compact mode",
					Description: "Use compact layout with less spacing",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
				{
					Key:         "confirm_destructive",
					Label:       "Confirm destructive",
					Description: "Confirm before destructive actions",
					Type:        OptionTypeBool,
					Options:     []string{"true", "false"},
				},
			},
		},
	}
}

func (m *OptionsModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *OptionsModalModel) Open(cfg *config.Config) {
	m.visible = true
	m.config = cfg
	m.originalConfig = cfg.Clone()
	m.sectionCursor = 0
	m.optionCursor = 0
	m.modified = false
}

func (m *OptionsModalModel) Close() {
	m.visible = false
}

func (m OptionsModalModel) Visible() bool {
	return m.visible
}

func (m OptionsModalModel) Update(msg tea.Msg) (OptionsModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "left", "h":
			m.cycleValue(-1)
		case "right", "l":
			m.cycleValue(1)
		case "s":
			return m.save()
		case "esc", "q":
			m.visible = false
			m.config = m.originalConfig
			return m, func() tea.Msg {
				return optionsCancelledMsg{}
			}
		}
	}

	return m, nil
}

func (m *OptionsModalModel) moveCursor(direction int) {
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
	for i := 0; i < m.sectionCursor; i++ {
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
	case "theme":
		if m.config.Theme == "" {
			return "default"
		}
		return m.config.Theme
	case "compact_mode":
		return boolStr(m.config.CompactMode)
	case "confirm_destructive":
		return boolStr(m.config.ConfirmDestructive)
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
		SetShowHints(m.config.ShowHints)
	case "theme":
		m.config.Theme = value
		if value == "dark" {
			SetTheme(DarkTheme)
		} else {
			SetTheme(DefaultTheme)
		}
	case "compact_mode":
		m.config.CompactMode = value == "true"
	case "confirm_destructive":
		m.config.ConfirmDestructive = value == "true"
	}
}

func (m OptionsModalModel) save() (OptionsModalModel, tea.Cmd) {
	cfg := m.config
	m.visible = false
	m.modified = false
	return m, func() tea.Msg {
		if err := cfg.Save(); err != nil {
			return optionsCancelledMsg{}
		}
		return optionsSavedMsg{config: cfg}
	}
}

func (m OptionsModalModel) View() string {
	if !m.visible {
		return ""
	}

	t := GetTheme()

	modalWidth := 54
	modalHeight := m.calculateModalHeight()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(modalWidth).
		Padding(1, 2)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Options"))
	b.WriteString("\n\n")

	flatIndex := 0
	currentFlat := m.flatIndex()

	for _, section := range m.sections {
		b.WriteString(dimStyle.Render(section.Title))
		b.WriteString("\n")

		for _, opt := range section.Options {
			cursor := "  "
			style := normalStyle
			valueStyle := dlssStyle

			if flatIndex == currentFlat {
				cursor = "> "
				style = selectedStyle
			}

			value := m.getConfigValue(opt.Key)
			label := fmt.Sprintf("%s%-22s: ", cursor, opt.Label)
			b.WriteString(style.Render(label))
			b.WriteString(valueStyle.Render(value))
			b.WriteString("\n")
			flatIndex++
		}
		b.WriteString("\n")
	}

	currentOption := m.getCurrentOption()
	if currentOption != nil {
		b.WriteString(dimStyle.Render(currentOption.Description))
		b.WriteString("\n")
	}

	if m.modified {
		b.WriteString(warningStyle.Render("(modified)"))
		b.WriteString("\n")
	}

	if hint := RenderHint("\n↑↓:navigate • ←→:change • s:save • esc:close"); hint != "" {
		b.WriteString(hint)
	}

	modal := boxStyle.Render(b.String())

	centerX := (m.width - modalWidth - 4) / 2
	centerY := (m.height - modalHeight - 4) / 2
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
