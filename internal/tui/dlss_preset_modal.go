package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/profile"
)

// dlssPresetOrder defines the display order for DLSS presets. The slice is
// expanded at package init through dedupePresets so the invariant "every
// preset appears at most once" holds even if duplicates are accidentally
// introduced upstream (Task 5 acceptance). Callers (modal, tests) must
// consult dlssPresets() rather than referencing this raw slice directly.
var dlssPresetOrderRaw = []profile.DLSSPreset{
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

// dlssPresetOrder is the deduplicated, order-preserving preset list rendered
// by the DLSS preset modal. Task 5 fix: if dlssPresetOrderRaw ever gains a
// duplicate entry (regression or accidental merge conflict), the picker
// still shows each preset exactly once.
var dlssPresetOrder = dedupePresets(dlssPresetOrderRaw)

// dedupePresets returns a new slice preserving first-seen order with
// duplicate preset keys removed. Exported-for-test as a package helper so
// a regression test can feed in an adversarial list with duplicates and
// assert the output is clean.
func dedupePresets(in []profile.DLSSPreset) []profile.DLSSPreset {
	seen := make(map[profile.DLSSPreset]struct{}, len(in))
	out := make([]profile.DLSSPreset, 0, len(in))
	for _, p := range in {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

type DLSSPresetModalModel struct {
	styles        *Styles
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

func NewDLSSPresetModal(styles *Styles) DLSSPresetModalModel {
	return DLSSPresetModalModel{styles: styles}
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

	s := m.styles
	t := s.Theme

	modalWidth := 70
	modalHeight := len(dlssPresetOrder) + 12

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(modalWidth).
		Padding(1, 2)

	var b strings.Builder

	b.WriteString(s.Title.Render("Select DLSS preset"))
	b.WriteString("\n\n")

	headerStyle := s.Dim.Bold(true)
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-10s %-12s %-14s", "Preset", "Version", "Technology")))
	b.WriteString("\n")

	for i, preset := range dlssPresetOrder {
		cursor := "  "
		style := s.Normal
		valueStyle := s.DLSS

		if i == m.cursor {
			cursor = "> "
			style = s.Selected
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
	b.WriteString(s.Dim.Render(description))
	b.WriteString("\n")

	if hint := s.RenderHint("\n\n" + "↑↓:navigate • enter:select • esc:cancel"); hint != "" {
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
