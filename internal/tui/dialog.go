package tui

import tea "charm.land/bubbletea/v2"

// Dialog is the interface for modal dialogs that overlay the main view.
// Dialogs handle their own key routing, rendering, and lifecycle.
type Dialog interface {
	// Visible reports whether the dialog is currently open.
	Visible() bool

	// SetSize informs the dialog of the available terminal dimensions.
	SetSize(width, height int)

	// Update processes a message and returns the (possibly modified) dialog
	// along with any command to execute.
	Update(msg tea.Msg) (Dialog, tea.Cmd)

	// View renders the dialog content.
	View() string
}
