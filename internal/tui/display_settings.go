package tui

import "github.com/jgabor/spela/internal/config"

// confirmsUpdates reports whether DLL updates ask first. Restore and install
// always ask, whatever Confirm destructive actions says.
func (m LayoutModel) confirmsUpdates() bool {
	return m.config == nil || m.config.ConfirmDestructive
}

// applyDisplaySettings carries Confirm destructive actions and Compact mode
// into the views. previous is the configuration before a save, or nil at
// startup. The layout follows Compact mode only at startup and when that
// setting changes, so a layout picked from Actions stays until then.
func (m *LayoutModel) applyDisplaySettings(previous *config.Config) {
	skip := !m.confirmsUpdates()
	m.pane.content.skipUpdateConfirmation = skip
	m.pane.dllsResource.skipUpdateConfirmation = skip
	if m.config == nil || previous != nil && previous.CompactMode == m.config.CompactMode {
		return
	}
	switch {
	case m.config.CompactMode:
		m.densityMode = DensityCompact
	case previous != nil:
		m.densityMode = DensityStandard
	}
	m.calculateDimensions()
}
