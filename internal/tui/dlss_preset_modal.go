package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/profile"
)

type DLSSPresetInfo struct {
	Preset      profile.DLSSPreset
	Version     string
	Technology  string
	Description string
}

var dlssPresets = []DLSSPresetInfo{
	{
		Preset:      profile.DLSSPresetDefault,
		Version:     "-",
		Technology:  "-",
		Description: "Use game's default preset",
	},
	{
		Preset:      profile.DLSSPresetA,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Basic preset for Performance, Balanced, Quality tiers. For games without all native DLSS inputs like motion vectors",
	},
	{
		Preset:      profile.DLSSPresetB,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Variant of A, improves Ultra Performance tier at high resolutions (4K+)",
	},
	{
		Preset:      profile.DLSSPresetC,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Variant of A for fast-paced games. Less temporally stable images, but less ghosting",
	},
	{
		Preset:      profile.DLSSPresetD,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Variant of A for slower-paced games. More temporally stable images, but more ghosting",
	},
	{
		Preset:      profile.DLSSPresetE,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Improved version of D, should be used over D in most cases",
	},
	{
		Preset:      profile.DLSSPresetF,
		Version:     "DLSS 2/3",
		Technology:  "CNN",
		Description: "Optimized for high resolutions (4K+) in Ultra Performance / DLAA quality tiers",
	},
	{
		Preset:      profile.DLSSPresetJ,
		Version:     "DLSS 4",
		Technology:  "Transformer",
		Description: "Baseline transformer preset. Sharper but less temporally stable than K",
	},
	{
		Preset:      profile.DLSSPresetK,
		Version:     "DLSS 4",
		Technology:  "Transformer",
		Description: "Variant of J. Blurrier but more temporally stable than J",
	},
	{
		Preset:      profile.DLSSPresetL,
		Version:     "DLSS 4.5",
		Technology:  "Transformer 2",
		Description: "Optimized for high resolutions (4K+) in Ultra Performance / DLAA quality tiers",
	},
	{
		Preset:      profile.DLSSPresetM,
		Version:     "DLSS 4.5",
		Technology:  "Transformer 2",
		Description: "Optimized for lower resolutions in Performance / Balanced / Quality tiers",
	},
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
	for i, p := range dlssPresets {
		if p.Preset == currentPreset {
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
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(dlssPresets)-1 {
				m.cursor++
			}
		case "enter":
			m.visible = false
			selectedPreset := dlssPresets[m.cursor].Preset
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
	modalHeight := len(dlssPresets) + 12

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

	for i, preset := range dlssPresets {
		cursor := "  "
		style := normalStyle
		valueStyle := dlssStyle

		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		presetName := string(preset.Preset)
		if preset.Preset == profile.DLSSPresetDefault {
			presetName = "(default)"
		}

		line := fmt.Sprintf("%s%-10s", cursor, presetName)
		b.WriteString(style.Render(line))
		b.WriteString(valueStyle.Render(fmt.Sprintf(" %-12s %-14s", preset.Version, preset.Technology)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	currentPreset := dlssPresets[m.cursor]
	b.WriteString(dimStyle.Render(currentPreset.Description))
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
