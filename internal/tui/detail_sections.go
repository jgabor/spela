package tui

import (
	"fmt"

	"github.com/jgabor/spela/internal/profile"
)

// CurrentSection returns the subsystem key of the focused field, or "".
func (m DetailModel) CurrentSection() string {
	descriptor, ok := profile.Field(m.FocusedField())
	if !ok {
		return ""
	}
	return descriptor.Subsystem
}

// CanJumpSection reports whether a section exists in that direction.
func (m DetailModel) CanJumpSection(delta int) bool {
	for index, section := range detailSectionOrder {
		if section == m.CurrentSection() {
			target := index + delta
			return target >= 0 && target < len(detailSectionOrder)
		}
	}
	return false
}

// JumpSection moves the cursor to the first field of the previous (delta < 0)
// or next (delta > 0) section. It reports whether the cursor moved. It never
// wraps, so the caller can hand over to the neighbouring view at an end.
func (m *DetailModel) JumpSection(delta int) bool {
	current := -1
	for index, section := range detailSectionOrder {
		if section == m.CurrentSection() {
			current = index
			break
		}
	}
	target := current + delta
	if current < 0 || target < 0 || target >= len(detailSectionOrder) {
		return false
	}
	return m.FocusSection(detailSectionOrder[target])
}

// FocusSection moves the cursor to the first field of a section.
func (m *DetailModel) FocusSection(section string) bool {
	for index, row := range m.focusableRows {
		descriptor, ok := profile.Field(m.rows[row].field)
		if ok && descriptor.Subsystem == section {
			m.cursor = index
			return true
		}
	}
	return false
}

// OwnSettings lists "label value" for every setting this game profile sets
// itself, in page order. The All games page has no own settings.
func (m DetailModel) OwnSettings() []string {
	if m.isRoot {
		return nil
	}
	var own []string
	for _, row := range m.focusableRows {
		field := m.rows[row].field
		value, _, isOwn := m.fieldText(field)
		if isOwn {
			own = append(own, m.rows[row].label+" "+value)
		}
	}
	return own
}

// SaveStatus is the one-line save state shown in the page header.
func (m DetailModel) SaveStatus() string {
	switch {
	case m.saveError != nil:
		return fmt.Sprintf("Not saved: %v", m.saveError)
	case m.Dirty():
		return "Saving…"
	default:
		return "Saved at once"
	}
}

// ClearableCount is the number of settings a whole-page Follow would clear:
// the settings a game sets itself, or the values All games has set.
func (m DetailModel) ClearableCount() int {
	if !m.isRoot {
		return len(m.OwnSettings())
	}
	count := 0
	for _, field := range profile.AllFields() {
		if !m.rootFieldAtDefault(field) {
			count++
		}
	}
	return count
}
