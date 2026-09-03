package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/game"
)

// dllMutationConfirmation is the single confirmation contract for every TUI
// operation that can write a game DLL.
type dllMutationConfirmation struct {
	title  string
	items  []string
	backup string
}

func newDLLMutationConfirmation(title string, games []*game.Game, target, backup string) *dllMutationConfirmation {
	items := make([]string, 0)
	for _, entry := range games {
		for _, installed := range entry.DLLs {
			version := installed.Version
			if target != "" {
				version += " → " + target
			}
			items = append(items, fmt.Sprintf("%s — %s — %s", entry.Name, installed.Path, version))
		}
	}
	return &dllMutationConfirmation{title: title, items: items, backup: backup}
}

func (confirmation *dllMutationConfirmation) update(key tea.KeyPressMsg) (confirm, cancel bool) {
	if confirmation == nil {
		return false, false
	}
	switch key.String() {
	case "enter", "y", "Y":
		return true, false
	case "esc", "escape", "q", "n", "N":
		return false, true
	}
	return false, false
}

func (confirmation *dllMutationConfirmation) view(styles *Styles) string {
	if confirmation == nil {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(styles.Title.Render(confirmation.title))
	builder.WriteString("\n\n")
	for _, item := range confirmation.items {
		builder.WriteString("• " + item + "\n")
	}
	builder.WriteString("\nBackup: " + confirmation.backup)
	builder.WriteString("\n\nEnter/Y confirm • Esc/q cancel")
	return builder.String()
}
