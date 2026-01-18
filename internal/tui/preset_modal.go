package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/profile"
)

var presetList = []profile.Preset{
	profile.PresetPerformance,
	profile.PresetBalanced,
	profile.PresetQuality,
	profile.PresetCustom,
}

type PresetModalModel struct {
	presets        []profile.Preset
	cursor         int
	originalPreset profile.Preset
	previewProfile *profile.Profile
	visible        bool
	width          int
	height         int
}

type presetSelectedMsg struct {
	preset profile.Preset
}

type presetCancelledMsg struct{}

func NewPresetModal() PresetModalModel {
	return PresetModalModel{
		presets: presetList,
	}
}

func (m *PresetModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *PresetModalModel) Open(currentPreset profile.Preset) {
	m.visible = true
	m.originalPreset = currentPreset
	m.cursor = 0
	for i, p := range m.presets {
		if p == currentPreset {
			m.cursor = i
			break
		}
	}
	m.previewProfile = profile.FromPreset(m.presets[m.cursor])
}

func (m *PresetModalModel) Close() {
	m.visible = false
}

func (m PresetModalModel) Visible() bool {
	return m.visible
}

func (m *PresetModalModel) updatePreview() {
	m.previewProfile = profile.FromPreset(m.presets[m.cursor])
}

func (m PresetModalModel) Update(msg tea.Msg) (PresetModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updatePreview()
			}
		case "down", "j":
			if m.cursor < len(m.presets)-1 {
				m.cursor++
				m.updatePreview()
			}
		case "enter":
			selectedPreset := m.presets[m.cursor]
			m.visible = false
			return m, func() tea.Msg {
				return presetSelectedMsg{preset: selectedPreset}
			}
		case "esc", "q":
			m.visible = false
			return m, func() tea.Msg {
				return presetCancelledMsg{}
			}
		}
	}

	return m, nil
}

func (m PresetModalModel) View() string {
	if !m.visible {
		return ""
	}

	t := GetTheme()

	modalWidth := 50
	modalHeight := len(m.presets) + 12

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(modalWidth).
		Padding(1, 2)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Select preset"))
	b.WriteString("\n\n")

	for i, preset := range m.presets {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		label := string(preset)
		if i == m.cursor && preset != m.originalPreset {
			label += " (preview)"
		}

		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, label)))
		b.WriteString("\n")
	}

	if m.previewProfile != nil {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Settings preview:"))
		b.WriteString("\n")
		b.WriteString(dlssStyle.Render(fmt.Sprintf("  SR mode: %s", m.previewProfile.DLSS.SRMode)))
		b.WriteString("\n")
		b.WriteString(dlssStyle.Render(fmt.Sprintf("  Frame gen: %v", m.previewProfile.DLSS.FGEnabled)))
		b.WriteString("\n")
		b.WriteString(dlssStyle.Render(fmt.Sprintf("  Power mode: %s", powerMizerDisplay(m.previewProfile.GPU.PowerMizer))))
		b.WriteString("\n")
	}

	if hint := RenderHint("\n↑↓ select • enter confirm • esc cancel"); hint != "" {
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

func powerMizerDisplay(p string) string {
	if p == "" {
		return "auto"
	}
	return p
}
