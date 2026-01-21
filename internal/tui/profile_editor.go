package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type ProfileEditorModel struct {
	game       *game.Game
	profile    *profile.Profile
	saveTarget ProfileSaveTarget
	cursor     int
	fields     []profileField
	modified   bool
	message    string
}

type profileField struct {
	label       string
	key         string
	value       string
	options     []string
	description string
}

func frameGenerationValue(p *profile.Profile) string {
	if !p.DLSS.FGOverride {
		return "(default)"
	}
	if p.DLSS.FGEnabled {
		return "true"
	}
	return "false"
}

func NewProfileEditor(g *game.Game, p *profile.Profile) ProfileEditorModel {
	if p == nil {
		p = &profile.Profile{Name: g.Name}
	}

	fields := []profileField{
		{
			label:       "Quality mode",
			key:         "sr_mode",
			value:       string(p.DLSS.SRMode),
			options:     []string{"off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
			description: "Super resolution quality. Higher = sharper but slower. DLAA is native res anti-aliasing",
		},
		{
			label:       "DLSS preset",
			key:         "sr_preset",
			value:       srPresetValue(p.DLSS.SRPreset),
			options:     []string{"default", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
			description: "Neural network preset (A-F: CNN, J-M: Transformer)",
		},
		{
			label:       "DLSS-SR Override",
			key:         "sr_override",
			value:       boolStr(p.DLSS.SROverride),
			options:     []string{"true", "false"},
			description: "Force DLSS super resolution even if game doesn't natively support it",
		},
		{
			label:       "Frame Gen",
			key:         "fg_enabled",
			value:       frameGenerationValue(p),
			options:     []string{"(default)", "true", "false"},
			description: "Generate extra frames using AI. Increases FPS but adds slight latency",
		},
		{
			label:       "Multi-Frame",
			key:         "multi_frame",
			value:       intStr(p.DLSS.MultiFrame),
			options:     []string{"0", "1", "2", "3", "4"},
			description: "Number of extra frames to generate (0=disabled, 1-4=frame multiplier)",
		},
		{
			label:       "DLSS Indicator",
			key:         "indicator",
			value:       boolStr(p.DLSS.Indicator),
			options:     []string{"true", "false"},
			description: "Show on-screen indicator when DLSS is active",
		},
		{
			label:       "Shader Cache",
			key:         "shader_cache",
			value:       boolStr(p.GPU.ShaderCache),
			options:     []string{"true", "false"},
			description: "Enable GPU shader caching for faster load times after first run",
		},
		{
			label:       "Threaded Opt",
			key:         "threaded_opt",
			value:       boolStr(p.GPU.ThreadedOptimization),
			options:     []string{"true", "false"},
			description: "Enable NVIDIA threaded optimization for multi-core performance",
		},
		{
			label:       "Power Mode",
			key:         "power_mizer",
			value:       powerMizerValue(p.GPU.PowerMizer),
			options:     []string{"auto", "adaptive", "max"},
			description: "GPU power mode: auto (driver decides), adaptive, max performance",
		},
		{
			label:       "HDR",
			key:         "hdr",
			value:       boolStr(p.Proton.EnableHDR),
			options:     []string{"true", "false"},
			description: "Enable high dynamic range output for compatible displays",
		},
		{
			label:       "Wayland",
			key:         "wayland",
			value:       boolStr(p.Proton.EnableWayland),
			options:     []string{"true", "false"},
			description: "Use native Wayland instead of XWayland. May improve latency",
		},
		{
			label:       "NGX Updater",
			key:         "ngx_updater",
			value:       boolStr(p.Proton.EnableNGXUpdater),
			options:     []string{"true", "false"},
			description: "Let Proton automatically update DLSS DLLs to latest version",
		},
		{
			label:       "Save Backup",
			key:         "backup_on_launch",
			value:       boolStr(p.Ludusavi.BackupOnLaunch),
			options:     []string{"true", "false"},
			description: "Automatically backup save games when launching via Ludusavi",
		},
	}

	return ProfileEditorModel{
		game:       g,
		profile:    p,
		saveTarget: NewGameProfileSaveTarget(g.AppID),
		fields:     fields,
	}
}

func NewDefaultProfileEditor(p *profile.Profile) ProfileEditorModel {
	if p == nil {
		p = &profile.Profile{Name: "Default profile"}
	}

	fields := []profileField{
		{
			label:       "Quality mode",
			key:         "sr_mode",
			value:       string(p.DLSS.SRMode),
			options:     []string{"off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
			description: "Super resolution quality. Higher = sharper but slower. DLAA is native res anti-aliasing",
		},
		{
			label:       "DLSS preset",
			key:         "sr_preset",
			value:       srPresetValue(p.DLSS.SRPreset),
			options:     []string{"default", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
			description: "Neural network preset (A-F: CNN, J-M: Transformer)",
		},
		{
			label:       "DLSS-SR Override",
			key:         "sr_override",
			value:       boolStr(p.DLSS.SROverride),
			options:     []string{"true", "false"},
			description: "Force DLSS super resolution even if game doesn't natively support it",
		},
		{
			label:       "Frame Gen",
			key:         "fg_enabled",
			value:       frameGenerationValue(p),
			options:     []string{"(default)", "true", "false"},
			description: "Generate extra frames using AI. Increases FPS but adds slight latency",
		},
		{
			label:       "Multi-Frame",
			key:         "multi_frame",
			value:       intStr(p.DLSS.MultiFrame),
			options:     []string{"0", "1", "2", "3", "4"},
			description: "Number of extra frames to generate (0=disabled, 1-4=frame multiplier)",
		},
		{
			label:       "DLSS Indicator",
			key:         "indicator",
			value:       boolStr(p.DLSS.Indicator),
			options:     []string{"true", "false"},
			description: "Show on-screen indicator when DLSS is active",
		},
		{
			label:       "Shader Cache",
			key:         "shader_cache",
			value:       boolStr(p.GPU.ShaderCache),
			options:     []string{"true", "false"},
			description: "Enable GPU shader caching for faster load times after first run",
		},
		{
			label:       "Threaded Opt",
			key:         "threaded_opt",
			value:       boolStr(p.GPU.ThreadedOptimization),
			options:     []string{"true", "false"},
			description: "Enable NVIDIA threaded optimization for multi-core performance",
		},
		{
			label:       "Power Mode",
			key:         "power_mizer",
			value:       powerMizerValue(p.GPU.PowerMizer),
			options:     []string{"auto", "adaptive", "max"},
			description: "GPU power mode: auto (driver decides), adaptive, max performance",
		},
		{
			label:       "HDR",
			key:         "hdr",
			value:       boolStr(p.Proton.EnableHDR),
			options:     []string{"true", "false"},
			description: "Enable high dynamic range output for compatible displays",
		},
		{
			label:       "Wayland",
			key:         "wayland",
			value:       boolStr(p.Proton.EnableWayland),
			options:     []string{"true", "false"},
			description: "Use native Wayland instead of XWayland. May improve latency",
		},
		{
			label:       "NGX Updater",
			key:         "ngx_updater",
			value:       boolStr(p.Proton.EnableNGXUpdater),
			options:     []string{"true", "false"},
			description: "Let Proton automatically update DLSS DLLs to latest version",
		},
		{
			label:       "Save Backup",
			key:         "backup_on_launch",
			value:       boolStr(p.Ludusavi.BackupOnLaunch),
			options:     []string{"true", "false"},
			description: "Backup saves when launching game",
		},
	}

	return ProfileEditorModel{
		profile:    p,
		saveTarget: DefaultProfileSaveTarget(),
		fields:     fields,
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func srPresetValue(p profile.DLSSPreset) string {
	if p == "" {
		return "default"
	}
	return string(p)
}

func intStr(i int) string {
	return fmt.Sprintf("%d", i)
}

func powerMizerValue(p string) string {
	if p == "" {
		return "auto"
	}
	return p
}

func (m ProfileEditorModel) Init() tea.Cmd {
	return nil
}

func (m ProfileEditorModel) Update(msg tea.Msg) (ProfileEditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.fields)-1 {
				m.cursor++
			}
		case "left", "h":
			m.cycleValue(-1)
		case "right", "l":
			m.cycleValue(1)
		case "s":
			return m, m.save()
		}
	}
	return m, nil
}

func (m *ProfileEditorModel) cycleValue(direction int) {
	field := &m.fields[m.cursor]
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

func (m *ProfileEditorModel) applyToProfile() {
	for _, f := range m.fields {
		switch f.key {
		case "sr_mode":
			m.profile.DLSS.SRMode = profile.DLSSMode(f.value)
		case "sr_preset":
			m.profile.DLSS.SRPreset = profile.DLSSPreset(f.value)
		case "sr_override":
			m.profile.DLSS.SROverride = f.value == "true"
		case "fg_enabled":
			if f.value == "(default)" {
				m.profile.DLSS.FGEnabled = false
				m.profile.DLSS.FGOverride = false
			} else {
				m.profile.DLSS.FGEnabled = f.value == "true"
				m.profile.DLSS.FGOverride = true
			}
		case "multi_frame":
			var v int
			_, _ = fmt.Sscanf(f.value, "%d", &v)
			m.profile.DLSS.MultiFrame = v
		case "indicator":
			m.profile.DLSS.Indicator = f.value == "true"
		case "shader_cache":
			m.profile.GPU.ShaderCache = f.value == "true"
		case "threaded_opt":
			m.profile.GPU.ThreadedOptimization = f.value == "true"
		case "power_mizer":
			if f.value == "auto" {
				m.profile.GPU.PowerMizer = ""
			} else {
				m.profile.GPU.PowerMizer = f.value
			}
		case "hdr":
			m.profile.Proton.EnableHDR = f.value == "true"
		case "wayland":
			m.profile.Proton.EnableWayland = f.value == "true"
		case "ngx_updater":
			m.profile.Proton.EnableNGXUpdater = f.value == "true"
		case "backup_on_launch":
			m.profile.Ludusavi.BackupOnLaunch = f.value == "true"
		}
	}
}

func (m ProfileEditorModel) save() tea.Cmd {
	return func() tea.Msg {
		if err := m.saveTarget.SaveProfile(m.profile); err != nil {
			return profileSaveMsg{err: err}
		}
		return profileSaveMsg{success: true}
	}
}

type profileSaveMsg struct {
	success bool
	err     error
}

func (m ProfileEditorModel) View() string {
	var b strings.Builder

	profileName := "Default profile"
	if m.game != nil {
		profileName = m.game.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Profile: %s", profileName)))
	b.WriteString("\n\n")

	for i, field := range m.fields {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%-16s: ", cursor, field.label)
		b.WriteString(style.Render(line))
		b.WriteString(dlssStyle.Render(field.value))
		b.WriteString("\n")
	}

	if m.modified {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("(modified)"))
	}

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render(m.message))
	}

	b.WriteString(helpStyle.Render("\n\n←/→ change value • s save • esc back"))

	return b.String()
}

func (m ProfileEditorModel) Modified() bool {
	return m.modified
}

func (m *ProfileEditorModel) SetMessage(msg string) {
	m.message = msg
}
