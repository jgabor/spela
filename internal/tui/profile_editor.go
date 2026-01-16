package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type ProfileEditorModel struct {
	game     *game.Game
	profile  *profile.Profile
	cursor   int
	fields   []profileField
	modified bool
	message  string
}

type profileField struct {
	label       string
	key         string
	value       string
	options     []string
	description string
}

func NewProfileEditor(g *game.Game, p *profile.Profile) ProfileEditorModel {
	if p == nil {
		p = profile.FromPreset(profile.PresetBalanced)
		p.Name = g.Name
	}

	fields := []profileField{
		{
			label:       "Preset",
			key:         "preset",
			value:       string(p.Preset),
			options:     []string{"performance", "balanced", "quality", "custom"},
			description: "Quick preset: performance (max FPS), balanced (good mix), quality (best visuals)",
		},
		{
			label:       "DLSS-SR Mode",
			key:         "sr_mode",
			value:       string(p.DLSS.SRMode),
			options:     []string{"off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
			description: "Super resolution quality. Higher = sharper but slower. DLAA is native res anti-aliasing",
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
			value:       boolStr(p.DLSS.FGEnabled),
			options:     []string{"true", "false"},
			description: "Generate extra frames using AI. Increases FPS but adds slight latency",
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
	}

	return ProfileEditorModel{
		game:    g,
		profile: p,
		fields:  fields,
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
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

	if field.key == "preset" && field.value != "custom" {
		m.applyPreset(profile.Preset(field.value))
	} else {
		m.applyToProfile()
	}
}

func (m *ProfileEditorModel) applyPreset(preset profile.Preset) {
	p := profile.FromPreset(preset)
	m.profile = p

	for i := range m.fields {
		switch m.fields[i].key {
		case "preset":
			m.fields[i].value = string(p.Preset)
		case "sr_mode":
			m.fields[i].value = string(p.DLSS.SRMode)
		case "sr_override":
			m.fields[i].value = boolStr(p.DLSS.SROverride)
		case "fg_enabled":
			m.fields[i].value = boolStr(p.DLSS.FGEnabled)
		case "hdr":
			m.fields[i].value = boolStr(p.Proton.EnableHDR)
		case "wayland":
			m.fields[i].value = boolStr(p.Proton.EnableWayland)
		case "ngx_updater":
			m.fields[i].value = boolStr(p.Proton.EnableNGXUpdater)
		}
	}
}

func (m *ProfileEditorModel) applyToProfile() {
	for _, f := range m.fields {
		switch f.key {
		case "preset":
			m.profile.Preset = profile.Preset(f.value)
		case "sr_mode":
			m.profile.DLSS.SRMode = profile.DLSSMode(f.value)
		case "sr_override":
			m.profile.DLSS.SROverride = f.value == "true"
		case "fg_enabled":
			m.profile.DLSS.FGEnabled = f.value == "true"
			m.profile.DLSS.FGOverride = true
		case "hdr":
			m.profile.Proton.EnableHDR = f.value == "true"
		case "wayland":
			m.profile.Proton.EnableWayland = f.value == "true"
		case "ngx_updater":
			m.profile.Proton.EnableNGXUpdater = f.value == "true"
		}
	}
}

func (m ProfileEditorModel) save() tea.Cmd {
	return func() tea.Msg {
		if err := profile.Save(m.game.AppID, m.profile); err != nil {
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

	b.WriteString(titleStyle.Render(fmt.Sprintf("Profile: %s", m.game.Name)))
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
