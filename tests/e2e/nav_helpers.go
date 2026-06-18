//go:build e2e

package e2e

import (
	"strings"
	"time"
)

// FocusContext moves focus from Primary to the Context zone.
func (s *Session) FocusContext() error {
	return s.SendKeys("tab")
}

// FocusContent moves focus from Context to the Content zone.
func (s *Session) FocusContent() error {
	return s.SendKeys("tab")
}

// FocusPrimary returns focus to the Primary zone from Context or Content.
func (s *Session) FocusPrimary() error {
	return s.SendKeys("esc")
}

// SelectGame filters the library sidebar and confirms the matching game scope.
func (s *Session) SelectGame(name string) error {
	if err := s.FocusContext(); err != nil {
		return err
	}
	keys := []string{"/"}
	for _, r := range strings.ToLower(name) {
		keys = append(keys, string(r))
	}
	keys = append(keys, "enter", "enter")
	return s.SendKeys(keys...)
}

// SelectDefaultProfile selects the pinned "All games (default)" scope.
func (s *Session) SelectDefaultProfile() error {
	if err := s.FocusContext(); err != nil {
		return err
	}
	return s.SendKeys("enter")
}

// SelectAspectOverview switches to the Overview aspect (works from Content or Context).
func (s *Session) SelectAspectOverview() error {
	return s.SendKeys("1")
}

// SelectAspectProfile switches to the Profile aspect (works from Content or Context).
func (s *Session) SelectAspectProfile() error {
	return s.SendKeys("2")
}

// SelectAspectDLLs switches to the DLLs aspect (works from Content or Context).
func (s *Session) SelectAspectDLLs() error {
	return s.SendKeys("3")
}

// WaitForBreadcrumb waits until the status bar shows the given segment text.
func (s *Session) WaitForBreadcrumb(substr string, timeout time.Duration) error {
	return s.WaitForText(substr, timeout)
}
