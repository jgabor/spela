package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/profile"
)

// dlssPresetOrder defines the display order for DLSS presets.
var dlssPresetOrder = []profile.DLSSPreset{
	profile.DLSSPresetDefault,
	profile.DLSSPresetA,
	profile.DLSSPresetB,
	profile.DLSSPresetC,
	profile.DLSSPresetD,
	profile.DLSSPresetE,
	profile.DLSSPresetF,
	profile.DLSSPresetJ,
	profile.DLSSPresetK,
	profile.DLSSPresetL,
	profile.DLSSPresetM,
}

type DLSSPresetModalModel struct {
	visible       bool
	cursor        int
	currentPreset profile.DLSSPreset
	width         int
	height        int
}

type dlssPresetSelectedMsg struct {
	preset profile.DLSSPreset
}

type dlssPresetCancelledMsg struct{}

func NewDLSSPresetModal() DLSSPresetModalModel {
	return DLSSPresetModalModel{}
}

func (m *DLSSPresetModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DLSSPresetModalModel) Open(currentPreset profile.DLSSPreset) {
	m.visible = true
	m.currentPreset = currentPreset

	m.cursor = 0
	for i, p := range dlssPresetOrder {
		if p == currentPreset {
			m.cursor = i
			break
		}
	}
}

func (m DLSSPresetModalModel) Visible() bool {
	return m.visible
}

func (m DLSSPresetModalModel) Update(msg tea.Msg) (DLSSPresetModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(dlssPresetOrder)-1 {
				m.cursor++
			}
		case "enter":
			m.visible = false
			selectedPreset := dlssPresetOrder[m.cursor]
			return m, func() tea.Msg {
				return dlssPresetSelectedMsg{preset: selectedPreset}
			}
		case "esc", "q":
			m.visible = false
			return m, func() tea.Msg {
				return dlssPresetCancelledMsg{}
			}
		}
	}

	return m, nil
}

func (m DLSSPresetModalModel) View() string {
	if !m.visible {
		return ""
	}

	t := GetTheme()

	modalWidth := 70
	modalHeight := len(dlssPresetOrder) + 12

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(modalWidth).
		Padding(1, 2)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Select DLSS preset"))
	b.WriteString("\n\n")

	headerStyle := dimStyle.Bold(true)
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-10s %-12s %-14s", "Preset", "Version", "Technology")))
	b.WriteString("\n")

	for i, preset := range dlssPresetOrder {
		cursor := "  "
		style := normalStyle
		valueStyle := dlssStyle

		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		presetName := string(preset)
		if preset == profile.DLSSPresetDefault {
			presetName = "(default)"
		}

		info := profile.DLSSPresetInfo[preset]

		line := fmt.Sprintf("%s%-10s", cursor, presetName)
		b.WriteString(style.Render(line))
		b.WriteString(valueStyle.Render(fmt.Sprintf(" %-12s %-14s", info.Version, info.Technology)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	currentPreset := dlssPresetOrder[m.cursor]
	currentInfo := profile.DLSSPresetInfo[currentPreset]
	description := currentInfo.Description
	if currentPreset == profile.DLSSPresetDefault {
		description = "Use game's default preset"
	}
	b.WriteString(dimStyle.Render(description))
	b.WriteString("\n")

	if hint := RenderHint("\n\n" + "↑↓:navigate • enter:select • esc:cancel"); hint != "" {
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
