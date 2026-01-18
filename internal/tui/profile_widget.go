package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type WidgetField struct {
	label       string
	key         string
	value       string
	options     []string
	description string
	usesModal   bool
}

type WidgetGroup struct {
	title  string
	fields []WidgetField
}

type ProfileWidgetModel struct {
	game         *game.Game
	profile      *profile.Profile
	groups       []WidgetGroup
	focusedGroup int
	focusedField int
	editing      bool // true = editing fields within widget, false = navigating grid
	modified     bool
	width        int
	height       int
}

func displayValue(v string) string {
	if v == "" || v == "default" || v == "auto" {
		return "(default)"
	}
	return v
}

func displayBool(b bool) string {
	if !b {
		return "(default)"
	}
	return "true"
}

func displayInt(i int) string {
	if i == 0 {
		return "(default)"
	}
	return fmt.Sprintf("%d", i)
}

type openDLSSPresetModalMsg struct {
	currentPreset profile.DLSSPreset
}

func NewProfileWidget(g *game.Game, p *profile.Profile) ProfileWidgetModel {
	if p == nil {
		p = &profile.Profile{Name: g.Name}
	}

	groups := []WidgetGroup{
		{
			title: "DLSS settings",
			fields: []WidgetField{
				{
					label:       "Quality mode",
					key:         "sr_mode",
					value:       displayValue(string(p.DLSS.SRMode)),
					options:     []string{"(default)", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
					description: "Super resolution quality mode",
				},
				{
					label:       "DLSS preset",
					key:         "sr_preset",
					value:       displayValue(srPresetValue(p.DLSS.SRPreset)),
					options:     []string{"(default)", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
					description: "Neural network preset (A-F: CNN, J-M: Transformer)",
					usesModal:   true,
				},
				{
					label:       "Override",
					key:         "sr_override",
					value:       displayBool(p.DLSS.SROverride),
					options:     []string{"(default)", "true", "false"},
					description: "Force DLSS even if unsupported",
				},
				{
					label:       "Indicator",
					key:         "indicator",
					value:       displayBool(p.DLSS.Indicator),
					options:     []string{"(default)", "true", "false"},
					description: "Show on-screen DLSS indicator",
				},
				{
					label:       "Frame gen",
					key:         "fg_enabled",
					value:       displayBool(p.DLSS.FGEnabled),
					options:     []string{"(default)", "true", "false"},
					description: "Enable AI frame generation",
				},
				{
					label:       "Multi-frame",
					key:         "multi_frame",
					value:       displayInt(p.DLSS.MultiFrame),
					options:     []string{"(default)", "1", "2", "3", "4"},
					description: "Extra frames to generate (0=off)",
				},
			},
		},
		{
			title: "GPU settings",
			fields: []WidgetField{
				{
					label:       "Shader cache",
					key:         "shader_cache",
					value:       displayBool(p.GPU.ShaderCache),
					options:     []string{"(default)", "true", "false"},
					description: "Enable GPU shader caching",
				},
				{
					label:       "Threaded opt",
					key:         "threaded_opt",
					value:       displayBool(p.GPU.ThreadedOptimization),
					options:     []string{"(default)", "true", "false"},
					description: "Enable threaded optimization",
				},
				{
					label:       "Power mode",
					key:         "power_mizer",
					value:       displayValue(powerMizerValue(p.GPU.PowerMizer)),
					options:     []string{"(default)", "adaptive", "max"},
					description: "GPU power mode",
				},
			},
		},
		{
			title: "Proton settings",
			fields: []WidgetField{
				{
					label:       "HDR",
					key:         "hdr",
					value:       displayBool(p.Proton.EnableHDR),
					options:     []string{"(default)", "true", "false"},
					description: "Enable high dynamic range",
				},
				{
					label:       "Wayland",
					key:         "wayland",
					value:       displayBool(p.Proton.EnableWayland),
					options:     []string{"(default)", "true", "false"},
					description: "Use native Wayland",
				},
				{
					label:       "NGX updater",
					key:         "ngx_updater",
					value:       displayBool(p.Proton.EnableNGXUpdater),
					options:     []string{"(default)", "true", "false"},
					description: "Auto-update DLSS DLLs",
				},
			},
		},
		{
			title: "Backup settings",
			fields: []WidgetField{
				{
					label:       "Save backup",
					key:         "backup_on_launch",
					value:       displayBool(p.Ludusavi.BackupOnLaunch),
					options:     []string{"(default)", "true", "false"},
					description: "Backup saves on launch",
				},
			},
		},
	}

	return ProfileWidgetModel{
		game:         g,
		profile:      p,
		groups:       groups,
		focusedGroup: 0,
		focusedField: 0,
		editing:      false,
	}
}

func (m *ProfileWidgetModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ProfileWidgetModel) Update(msg tea.Msg) (ProfileWidgetModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateGrid(msg)
	}
	return m, nil
}

func (m ProfileWidgetModel) updateGrid(msg tea.KeyMsg) (ProfileWidgetModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.focusedGroup > 0 {
			m.focusedGroup--
		}
	case "down", "j":
		if m.focusedGroup < len(m.groups)-1 {
			m.focusedGroup++
		}
	case "left", "h":
		if m.focusedGroup > 0 {
			m.focusedGroup--
		}
	case "right", "l":
		if m.focusedGroup < len(m.groups)-1 {
			m.focusedGroup++
		}
	case "enter":
		m.editing = true
		m.focusedField = 0
	case "s":
		return m, m.save()
	}

	return m, nil
}

func (m ProfileWidgetModel) updateEditing(msg tea.KeyMsg) (ProfileWidgetModel, tea.Cmd) {
	group := &m.groups[m.focusedGroup]

	switch msg.String() {
	case "up", "k":
		if m.focusedField > 0 {
			m.focusedField--
		}
	case "down", "j":
		if m.focusedField < len(group.fields)-1 {
			m.focusedField++
		}
	case "left", "h":
		field := group.fields[m.focusedField]
		if !field.usesModal {
			m.cycleFieldValue(-1)
		}
	case "right", "l":
		field := group.fields[m.focusedField]
		if !field.usesModal {
			m.cycleFieldValue(1)
		}
	case "enter":
		field := group.fields[m.focusedField]
		if field.usesModal && field.key == "sr_preset" {
			currentPreset := profile.DLSSPreset(m.profile.DLSS.SRPreset)
			return m, func() tea.Msg {
				return openDLSSPresetModalMsg{currentPreset: currentPreset}
			}
		}
	case "esc", "q":
		m.editing = false
	case "s":
		return m, m.save()
	}

	return m, nil
}

func (m *ProfileWidgetModel) cycleFieldValue(direction int) {
	group := &m.groups[m.focusedGroup]
	field := &group.fields[m.focusedField]

	if len(field.options) == 0 {
		return
	}

	currentIndex := 0
	for i, opt := range field.options {
		if opt == field.value {
			currentIndex = i
			break
		}
	}

	newIndex := (currentIndex + direction + len(field.options)) % len(field.options)
	field.value = field.options[newIndex]
	m.modified = true
	m.applyToProfile()
}

func (m *ProfileWidgetModel) applyToProfile() {
	for _, group := range m.groups {
		for _, field := range group.fields {
			value := field.value
			isDefault := value == "(default)"

			switch field.key {
			case "sr_mode":
				if isDefault {
					m.profile.DLSS.SRMode = ""
				} else {
					m.profile.DLSS.SRMode = profile.DLSSMode(value)
				}
			case "sr_preset":
				if isDefault {
					m.profile.DLSS.SRPreset = ""
				} else {
					m.profile.DLSS.SRPreset = profile.DLSSPreset(value)
				}
			case "sr_override":
				m.profile.DLSS.SROverride = value == "true"
			case "fg_enabled":
				m.profile.DLSS.FGEnabled = value == "true"
				if !isDefault {
					m.profile.DLSS.FGOverride = true
				}
			case "multi_frame":
				if isDefault {
					m.profile.DLSS.MultiFrame = 0
				} else {
					var v int
					_, _ = fmt.Sscanf(value, "%d", &v)
					m.profile.DLSS.MultiFrame = v
				}
			case "indicator":
				m.profile.DLSS.Indicator = value == "true"
			case "shader_cache":
				m.profile.GPU.ShaderCache = value == "true"
			case "threaded_opt":
				m.profile.GPU.ThreadedOptimization = value == "true"
			case "power_mizer":
				if isDefault {
					m.profile.GPU.PowerMizer = ""
				} else {
					m.profile.GPU.PowerMizer = value
				}
			case "hdr":
				m.profile.Proton.EnableHDR = value == "true"
			case "wayland":
				m.profile.Proton.EnableWayland = value == "true"
			case "ngx_updater":
				m.profile.Proton.EnableNGXUpdater = value == "true"
			case "backup_on_launch":
				m.profile.Ludusavi.BackupOnLaunch = value == "true"
			}
		}
	}
}

func (m *ProfileWidgetModel) SetDLSSPreset(preset profile.DLSSPreset) {
	m.profile.DLSS.SRPreset = preset
	m.modified = true

	for gi := range m.groups {
		for fi := range m.groups[gi].fields {
			if m.groups[gi].fields[fi].key == "sr_preset" {
				if preset == "" || preset == profile.DLSSPresetDefault {
					m.groups[gi].fields[fi].value = "(default)"
				} else {
					m.groups[gi].fields[fi].value = string(preset)
				}
				return
			}
		}
	}
}

func (m ProfileWidgetModel) save() tea.Cmd {
	return func() tea.Msg {
		if err := profile.Save(m.game.AppID, m.profile); err != nil {
			return profileSaveMsg{err: err}
		}
		return profileSaveMsg{success: true}
	}
}

func (m ProfileWidgetModel) Modified() bool {
	return m.modified
}

func (m ProfileWidgetModel) Editing() bool {
	return m.editing
}

func (m ProfileWidgetModel) columnCount() int {
	if m.width >= 80 {
		return 2
	}
	return 1
}

func (m ProfileWidgetModel) View() string {
	var b strings.Builder

	t := GetTheme()
	sectionStyle := titleStyle.Foreground(t.Secondary)

	b.WriteString(sectionStyle.Render("Profile"))
	b.WriteString("\n")

	columns := m.columnCount()

	if columns == 2 {
		m.renderTwoColumn(&b)
	} else {
		m.renderSingleColumn(&b)
	}

	// Description of currently focused item
	description := m.getCurrentDescription()
	if description != "" {
		b.WriteString(dimStyle.Render("  " + description))
		b.WriteString("\n")
	} else {
		b.WriteString("\n") // Keep fixed height
	}

	if m.modified {
		b.WriteString(RenderHint("  (modified) s:save"))
		b.WriteString("\n")
	}

	var hint string
	if m.editing {
		hint = "  ↑↓:navigate • ←→:change • esc:back • s:save"
	} else {
		hint = "  ↑↓←→:navigate • enter:edit • s:save"
	}
	if h := RenderHint(hint); h != "" {
		b.WriteString(h)
		b.WriteString("\n")
	}

	return b.String()
}

func (m ProfileWidgetModel) getCurrentDescription() string {
	if m.focusedGroup >= len(m.groups) {
		return ""
	}
	group := m.groups[m.focusedGroup]

	if m.editing && m.focusedField < len(group.fields) {
		return group.fields[m.focusedField].description
	}

	// When not editing, show a summary of what the group contains
	switch group.title {
	case "DLSS settings":
		return "NVIDIA DLSS super resolution and frame generation settings"
	case "GPU settings":
		return "GPU driver and optimization settings"
	case "Proton settings":
		return "Proton compatibility layer settings"
	case "Backup settings":
		return "Game save backup settings via Ludusavi"
	}
	return ""
}

func (m ProfileWidgetModel) renderSingleColumn(b *strings.Builder) {
	widgetWidth := m.width - 4

	for gi, group := range m.groups {
		isWidgetFocused := gi == m.focusedGroup
		widget := m.renderWidgetBox(group, isWidgetFocused, widgetWidth)
		b.WriteString(widget)
		b.WriteString("\n")
	}
}

func (m ProfileWidgetModel) renderTwoColumn(b *strings.Builder) {
	columnWidth := (m.width - 8) / 2

	rows := (len(m.groups) + 1) / 2
	for row := range rows {
		leftIdx := row * 2
		rightIdx := row*2 + 1

		leftWidget := m.renderWidgetBox(m.groups[leftIdx], leftIdx == m.focusedGroup, columnWidth)
		rightWidget := ""
		if rightIdx < len(m.groups) {
			rightWidget = m.renderWidgetBox(m.groups[rightIdx], rightIdx == m.focusedGroup, columnWidth)
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftWidget, "  ", rightWidget))
		b.WriteString("\n")
	}
}

func (m ProfileWidgetModel) renderWidgetBox(group WidgetGroup, isWidgetFocused bool, width int) string {
	t := GetTheme()

	borderColor := t.Border
	if isWidgetFocused {
		borderColor = t.BorderFocus
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width-2).
		Padding(0, 1)

	var content strings.Builder

	// Widget title
	groupTitleStyle := titleStyle.Foreground(t.Secondary).MarginBottom(0)
	content.WriteString(groupTitleStyle.Render(group.title))
	content.WriteString("\n")

	// Fields
	for fi, field := range group.fields {
		isFieldFocused := isWidgetFocused && m.editing && fi == m.focusedField
		line := m.renderFieldToString(field, isFieldFocused)
		content.WriteString(line)
		content.WriteString("\n")
	}

	return boxStyle.Render(strings.TrimSuffix(content.String(), "\n"))
}

func (m ProfileWidgetModel) renderFieldToString(field WidgetField, isFieldFocused bool) string {
	prefix := "  "
	style := normalStyle
	valueStyle := dlssStyle

	if isFieldFocused {
		prefix = "> "
		style = selectedStyle
	}

	line := fmt.Sprintf("%s%-14s: ", prefix, field.label)
	result := style.Render(line) + valueStyle.Render(field.value)

	if isFieldFocused {
		var hint string
		if field.usesModal {
			hint = " enter:open"
		} else {
			hint = " ←→:change"
		}
		result += dimStyle.Render(hint)
	}

	return result
}
