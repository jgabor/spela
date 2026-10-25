package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/release"
)

// releaseNoticeMsg carries the header notice for a newer release.
type releaseNoticeMsg struct{ text string }

// checkForRelease looks for a newer release off the UI thread. A failed
// lookup stays silent: the TUI owns the terminal, and the next start retries
// after the check interval.
func checkForRelease(current string) tea.Cmd {
	return func() tea.Msg {
		notice, _ := release.Check(context.Background(), current, time.Now())
		if notice == nil {
			return nil
		}
		return releaseNoticeMsg{text: notice.Text()}
	}
}
