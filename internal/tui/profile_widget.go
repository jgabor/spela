package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	apply       func(p *profile.Profile, value string, isDefault bool)
}

type WidgetGroup struct {
	title  string
	fields []WidgetField
}

type ProfileWidgetModel struct {
	styles       *Styles
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

type openDLSSPresetModalMsg struct {
	currentPreset profile.DLSSPreset
}

type profileSaveMsg struct {
	success bool
	err     error
}

func NewProfileWidget(g *game.Game, p *profile.Profile, styles *Styles) ProfileWidgetModel {
	return newProfileWidget(NewGameProfileSaveTarget(g.AppID), g.Name, p, styles)
}

func NewDefaultProfileWidget(p *profile.Profile, styles *Styles) ProfileWidgetModel {
	return newProfileWidget(DefaultProfileSaveTarget(), "Default profile", p, styles)
}

func (m *ProfileWidgetModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ProfileWidgetModel) Update(msg tea.Msg) (ProfileWidgetModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateGrid(msg)
	}
	return m, nil
}

func (m ProfileWidgetModel) updateGrid(msg tea.KeyPressMsg) (ProfileWidgetModel, tea.Cmd) {
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

func (m ProfileWidgetModel) updateEditing(msg tea.KeyPressMsg) (ProfileWidgetModel, tea.Cmd) {
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
			if field.apply != nil {
				field.apply(m.profile, field.value, field.value == "(default)")
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
	s := m.styles
	var b strings.Builder

	sectionStyle := s.Title.Foreground(s.Theme.Secondary)

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
		b.WriteString(s.Dim.Render("  " + description))
		b.WriteString("\n")
	} else {
		b.WriteString("\n") // Keep fixed height
	}

	if m.modified {
		b.WriteString(s.RenderHint("  (modified) s:save"))
		b.WriteString("\n")
	}

	var hint string
	if m.editing {
		hint = "  ↑↓:navigate • ←→:change • esc:back • s:save"
	} else {
		hint = "  ↑↓←→:navigate • enter:edit • s:save"
	}
	if h := s.RenderHint(hint); h != "" {
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
	s := m.styles
	t := s.Theme

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
	groupTitleStyle := s.Title.Foreground(t.Secondary).MarginBottom(0)
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
	s := m.styles
	prefix := "  "
	style := s.Normal
	valueStyle := s.DLSS

	if field.disabled {
		line := fmt.Sprintf("%s%-14s: ", prefix, field.label)
		return s.Dim.Render(line + "Coming soon")
	}

	if isFieldFocused {
		prefix = "> "
		style = s.Selected
	}

	displayedValue := field.value
	isUnknown := len(displayedValue) > 0 && displayedValue[0] == '?'
	if isUnknown {
		valueStyle = s.Warning
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
		result += s.Dim.Render(hint)
	}

	return result
}
