package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type ProfileSaveTargetKind int

const (
	ProfileSaveTargetGame ProfileSaveTargetKind = iota
	ProfileSaveTargetDefault
)

type ProfileSaveTarget struct {
	kind  ProfileSaveTargetKind
	appID uint64
}

func NewGameProfileSaveTarget(appID uint64) ProfileSaveTarget {
	return ProfileSaveTarget{
		kind:  ProfileSaveTargetGame,
		appID: appID,
	}
}

func DefaultProfileSaveTarget() ProfileSaveTarget {
	return ProfileSaveTarget{kind: ProfileSaveTargetDefault}
}

func (target ProfileSaveTarget) SaveProfile(p *profile.Profile) error {
	switch target.kind {
	case ProfileSaveTargetDefault:
		return profile.SaveDefault(p)
	case ProfileSaveTargetGame:
		return profile.Save(target.appID, p)
	default:
		return fmt.Errorf("unsupported profile save target")
	}
}

type WidgetField struct {
	label       string
	key         string
	value       string
	options     []string
	description string
	usesModal   bool
	disabled    bool // field is visible but not yet functional
}

type WidgetGroup struct {
	title  string
	fields []WidgetField
}

type ProfileWidgetModel struct {
	profile      *profile.Profile
	saveTarget   ProfileSaveTarget
	groups       []WidgetGroup
	focusedGroup int
	focusedField int
	editing      bool // true = editing fields within widget, false = navigating grid
	modified     bool
	width        int
	height       int
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intStr(i int) string {
	return fmt.Sprintf("%d", i)
}

func srPresetValue(p profile.DLSSPreset) string {
	if p == "" {
		return "default"
	}
	return string(p)
}

func powerMizerValue(p string) string {
	if p == "" {
		return "auto"
	}
	return p
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

func displayBoolPtr(b *bool) string {
	if b == nil {
		return "(default)"
	}
	if *b {
		return "true"
	}
	return "false"
}

func displayFrameGeneration(enabled bool, override bool) string {
	if !override {
		return "(default)"
	}
	if enabled {
		return "true"
	}
	return "false"
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

type profileSaveMsg struct {
	success bool
	err     error
}

func NewProfileWidget(g *game.Game, p *profile.Profile) ProfileWidgetModel {
	return newProfileWidget(NewGameProfileSaveTarget(g.AppID), g.Name, p)
}

func NewDefaultProfileWidget(p *profile.Profile) ProfileWidgetModel {
	return newProfileWidget(DefaultProfileSaveTarget(), "Default profile", p)
}

func newProfileWidget(saveTarget ProfileSaveTarget, name string, p *profile.Profile) ProfileWidgetModel {
	if p == nil {
		p = &profile.Profile{Name: name}
	}

	groups := []WidgetGroup{
		{
			title: "DLSS super resolution",
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
					label:       "Model preset",
					key:         "sr_model_preset",
					value:       displayValue(string(p.DLSS.SRModelPreset)),
					options:     []string{"(default)", "auto", "k", "l", "m"},
					description: "Force specific transformer model version",
				},
				{
					label:       "Override",
					key:         "sr_override",
					value:       displayBool(p.DLSS.SROverride),
					options:     []string{"(default)", "true", "false"},
					description: "Force DLSS even if unsupported",
				},
				{
					label:       "SR indicator",
					key:         "indicator",
					value:       displayBool(p.DLSS.Indicator),
					options:     []string{"(default)", "true", "false"},
					description: "Show on-screen DLSS indicator",
				},
			},
		},
		{
			title: "DLSS ray reconstruction",
			fields: []WidgetField{
				{
					label:       "RR mode",
					key:         "rr_mode",
					value:       displayValue(string(p.DLSS.RRMode)),
					options:     []string{"(default)", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
					description: "Ray reconstruction quality mode",
				},
				{
					label:       "RR preset",
					key:         "rr_preset",
					value:       displayValue(srPresetValue(p.DLSS.RRPreset)),
					options:     []string{"(default)", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
					description: "Ray reconstruction neural network preset",
				},
				{
					label:       "RR override",
					key:         "rr_override",
					value:       displayBool(p.DLSS.RROverride),
					options:     []string{"(default)", "true", "false"},
					description: "Force ray reconstruction even if unsupported",
				},
			},
		},
		{
			title: "DLSS frame generation",
			fields: []WidgetField{
				{
					label:       "Frame gen",
					key:         "fg_enabled",
					value:       displayFrameGeneration(p.DLSS.FGEnabled, p.DLSS.FGOverride),
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
				{
					label:       "FG indicator",
					key:         "fg_indicator",
					value:       displayBool(p.DLSS.FGIndicator),
					options:     []string{"(default)", "true", "false"},
					description: "Show on-screen frame generation indicator",
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
					label:       "Shader cache path",
					key:         "shader_cache_path",
					value:       displayValue(p.GPU.ShaderCachePath),
					options:     nil,
					description: "Custom path for shader cache storage",
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
					disabled:    true,
				},
				{
					label:       "Clock offset",
					key:         "clock_offset",
					value:       displayInt(p.GPU.ClockOffset),
					options:     nil,
					description: "GPU core clock offset in MHz",
					disabled:    true,
				},
				{
					label:       "Memory offset",
					key:         "memory_offset",
					value:       displayInt(p.GPU.MemoryOffset),
					options:     nil,
					description: "GPU memory clock offset in MHz",
					disabled:    true,
				},
			},
		},
		{
			title: "CPU settings",
			fields: []WidgetField{
				{
					label:       "Governor",
					key:         "cpu_governor",
					value:       displayValue(p.CPU.Governor),
					options:     []string{"(default)", "performance", "powersave", "schedutil", "ondemand"},
					description: "CPU frequency scaling governor",
					disabled:    true,
				},
				{
					label:       "SMT",
					key:         "cpu_smt",
					value:       displayBoolPtr(p.CPU.SMT),
					options:     []string{"(default)", "true", "false"},
					description: "Enable simultaneous multi-threading (hyperthreading)",
					disabled:    true,
				},
				{
					label:       "Affinity",
					key:         "cpu_affinity",
					value:       displayValue(p.CPU.Affinity),
					options:     nil,
					description: "CPU core affinity mask (hex or decimal)",
					disabled:    true,
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
			title: "Overlay settings",
			fields: []WidgetField{
				{
					label:       "Enabled",
					key:         "overlay_enabled",
					value:       displayBool(p.Overlay.Enabled),
					options:     []string{"(default)", "true", "false"},
					description: "Show performance overlay",
					disabled:    true,
				},
				{
					label:       "Position",
					key:         "overlay_position",
					value:       displayValue(p.Overlay.Position),
					options:     []string{"(default)", "top-left", "top-right", "bottom-left", "bottom-right"},
					description: "Overlay screen position",
					disabled:    true,
				},
				{
					label:       "Show FPS",
					key:         "overlay_fps",
					value:       displayBool(p.Overlay.ShowFPS),
					options:     []string{"(default)", "true", "false"},
					description: "Show frames per second in overlay",
					disabled:    true,
				},
				{
					label:       "Show frametime",
					key:         "overlay_frametime",
					value:       displayBool(p.Overlay.ShowFrametime),
					options:     []string{"(default)", "true", "false"},
					description: "Show frame time in overlay",
					disabled:    true,
				},
				{
					label:       "Show CPU",
					key:         "overlay_cpu",
					value:       displayBool(p.Overlay.ShowCPU),
					options:     []string{"(default)", "true", "false"},
					description: "Show CPU usage in overlay",
					disabled:    true,
				},
				{
					label:       "Show GPU",
					key:         "overlay_gpu",
					value:       displayBool(p.Overlay.ShowGPU),
					options:     []string{"(default)", "true", "false"},
					description: "Show GPU usage in overlay",
					disabled:    true,
				},
				{
					label:       "Show VRAM",
					key:         "overlay_vram",
					value:       displayBool(p.Overlay.ShowVRAM),
					options:     []string{"(default)", "true", "false"},
					description: "Show VRAM usage in overlay",
					disabled:    true,
				},
				{
					label:       "Toggle key",
					key:         "overlay_toggle_key",
					value:       displayValue(p.Overlay.ToggleKey),
					options:     nil,
					description: "Key to toggle overlay visibility",
					disabled:    true,
				},
			},
		},
		{
			title: "Backup settings",
			fields: []WidgetField{
				{
					label:       "Backup on launch",
					key:         "backup_on_launch",
					value:       displayBool(p.Ludusavi.BackupOnLaunch),
					options:     []string{"(default)", "true", "false"},
					description: "Backup saves on launch",
				},
				{
					label:       "Restore on launch",
					key:         "restore_on_launch",
					value:       displayBool(p.Ludusavi.RestoreOnLaunch),
					options:     []string{"(default)", "true", "false"},
					description: "Restore saves on launch",
					disabled:    true,
				},
			},
		},
	}

	return ProfileWidgetModel{
		profile:      p,
		saveTarget:   saveTarget,
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
	cols := m.columnCount()
	switch msg.String() {
	case "up", "k":
		// Move up within the same column
		col := m.focusedGroup % cols
		prev := m.focusedGroup - cols
		if prev >= 0 && prev%cols == col {
			m.focusedGroup = prev
		}
	case "down", "j":
		// Move down within the same column
		col := m.focusedGroup % cols
		next := m.focusedGroup + cols
		if next < len(m.groups) && next%cols == col {
			m.focusedGroup = next
		}
	case "left", "h":
		// Move left to previous column (same row)
		if cols > 1 && m.focusedGroup%cols > 0 {
			m.focusedGroup--
		}
	case "right", "l":
		// Move right to next column (same row)
		if cols > 1 && m.focusedGroup%cols < cols-1 && m.focusedGroup+1 < len(m.groups) {
			m.focusedGroup++
		}
	case "enter":
		m.editing = true
		m.focusedField = 0
		for i, f := range m.groups[m.focusedGroup].fields {
			if !f.disabled {
				m.focusedField = i
				break
			}
		}
	case "s":
		return m, m.save()
	}

	return m, nil
}

func (m ProfileWidgetModel) updateEditing(msg tea.KeyMsg) (ProfileWidgetModel, tea.Cmd) {
	group := &m.groups[m.focusedGroup]

	switch msg.String() {
	case "up", "k":
		for i := m.focusedField - 1; i >= 0; i-- {
			if !group.fields[i].disabled {
				m.focusedField = i
				break
			}
		}
	case "down", "j":
		for i := m.focusedField + 1; i < len(group.fields); i++ {
			if !group.fields[i].disabled {
				m.focusedField = i
				break
			}
		}
	case "left", "h":
		field := group.fields[m.focusedField]
		if !field.disabled && !field.usesModal {
			m.cycleFieldValue(-1)
		}
	case "right", "l":
		field := group.fields[m.focusedField]
		if !field.disabled && !field.usesModal {
			m.cycleFieldValue(1)
		}
	case "enter":
		field := group.fields[m.focusedField]
		if field.disabled {
			break
		}
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

	// If the current value is unknown (prefixed with "?"), start from first option.
	rawValue := field.value
	if len(rawValue) > 0 && rawValue[0] == '?' {
		rawValue = field.options[0]
	}

	currentIndex := 0
	for i, opt := range field.options {
		if opt == rawValue {
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
			case "sr_model_preset":
				if isDefault {
					m.profile.DLSS.SRModelPreset = ""
				} else {
					m.profile.DLSS.SRModelPreset = profile.DLSSModelPreset(value)
				}
			case "sr_override":
				m.profile.DLSS.SROverride = value == "true"
			case "rr_mode":
				if isDefault {
					m.profile.DLSS.RRMode = ""
				} else {
					m.profile.DLSS.RRMode = profile.DLSSMode(value)
				}
			case "rr_preset":
				if isDefault {
					m.profile.DLSS.RRPreset = ""
				} else {
					m.profile.DLSS.RRPreset = profile.DLSSPreset(value)
				}
			case "rr_override":
				m.profile.DLSS.RROverride = value == "true"
			case "fg_enabled":
				if isDefault {
					m.profile.DLSS.FGEnabled = false
					m.profile.DLSS.FGOverride = false
				} else {
					m.profile.DLSS.FGEnabled = value == "true"
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
			case "fg_indicator":
				m.profile.DLSS.FGIndicator = value == "true"
			case "shader_cache":
				m.profile.GPU.ShaderCache = value == "true"
			case "shader_cache_path":
				if isDefault {
					m.profile.GPU.ShaderCachePath = ""
				} else {
					m.profile.GPU.ShaderCachePath = value
				}
			case "threaded_opt":
				m.profile.GPU.ThreadedOptimization = value == "true"
			case "power_mizer":
				if isDefault {
					m.profile.GPU.PowerMizer = ""
				} else {
					m.profile.GPU.PowerMizer = value
				}
			case "clock_offset":
				if isDefault {
					m.profile.GPU.ClockOffset = 0
				} else {
					var v int
					_, _ = fmt.Sscanf(value, "%d", &v)
					m.profile.GPU.ClockOffset = v
				}
			case "memory_offset":
				if isDefault {
					m.profile.GPU.MemoryOffset = 0
				} else {
					var v int
					_, _ = fmt.Sscanf(value, "%d", &v)
					m.profile.GPU.MemoryOffset = v
				}
			case "cpu_governor":
				if isDefault {
					m.profile.CPU.Governor = ""
				} else {
					m.profile.CPU.Governor = value
				}
			case "cpu_smt":
				if isDefault {
					m.profile.CPU.SMT = nil
				} else {
					b := value == "true"
					m.profile.CPU.SMT = &b
				}
			case "cpu_affinity":
				if isDefault {
					m.profile.CPU.Affinity = ""
				} else {
					m.profile.CPU.Affinity = value
				}
			case "hdr":
				m.profile.Proton.EnableHDR = value == "true"
			case "wayland":
				m.profile.Proton.EnableWayland = value == "true"
			case "ngx_updater":
				m.profile.Proton.EnableNGXUpdater = value == "true"
			case "overlay_enabled":
				m.profile.Overlay.Enabled = value == "true"
			case "overlay_position":
				if isDefault {
					m.profile.Overlay.Position = ""
				} else {
					m.profile.Overlay.Position = value
				}
			case "overlay_fps":
				m.profile.Overlay.ShowFPS = value == "true"
			case "overlay_frametime":
				m.profile.Overlay.ShowFrametime = value == "true"
			case "overlay_cpu":
				m.profile.Overlay.ShowCPU = value == "true"
			case "overlay_gpu":
				m.profile.Overlay.ShowGPU = value == "true"
			case "overlay_vram":
				m.profile.Overlay.ShowVRAM = value == "true"
			case "overlay_toggle_key":
				if isDefault {
					m.profile.Overlay.ToggleKey = ""
				} else {
					m.profile.Overlay.ToggleKey = value
				}
			case "backup_on_launch":
				m.profile.Ludusavi.BackupOnLaunch = value == "true"
			case "restore_on_launch":
				m.profile.Ludusavi.RestoreOnLaunch = value == "true"
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
		if err := m.saveTarget.SaveProfile(m.profile); err != nil {
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
	case "DLSS super resolution":
		return "NVIDIA DLSS super resolution quality and preset settings"
	case "DLSS ray reconstruction":
		return "NVIDIA DLSS ray reconstruction mode and preset settings"
	case "DLSS frame generation":
		return "NVIDIA DLSS frame generation and multi-frame settings"
	case "GPU settings":
		return "GPU driver and optimization settings"
	case "CPU settings":
		return "CPU governor, SMT, and affinity settings"
	case "Proton settings":
		return "Proton compatibility layer settings"
	case "Overlay settings":
		return "Performance overlay display settings"
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

	if field.disabled {
		line := fmt.Sprintf("%s%-14s: ", prefix, field.label)
		return dimStyle.Render(line + "Coming soon")
	}

	if isFieldFocused {
		prefix = "> "
		style = selectedStyle
	}

	displayedValue := field.value
	isUnknown := len(displayedValue) > 0 && displayedValue[0] == '?'
	if isUnknown {
		valueStyle = warningStyle
	}

	line := fmt.Sprintf("%s%-14s: ", prefix, field.label)
	result := style.Render(line) + valueStyle.Render(displayedValue)

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
