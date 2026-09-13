package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

type dllMutationTarget struct {
	appID          uint64
	gameName       string
	family         string
	manifestKey    string
	path           string
	currentVersion string
	targetVersion  string
}

func newDLLMutationTarget(entry *game.Game, installed game.DetectedDLL, targetVersion string) dllMutationTarget {
	return dllMutationTarget{
		appID:          entry.AppID,
		gameName:       entry.Name,
		family:         dllFamilyName(string(installed.Type)),
		manifestKey:    strings.ToLower(string(installed.Type)),
		path:           installed.Path,
		currentVersion: installed.Version,
		targetVersion:  targetVersion,
	}
}

func latestDLLMutationTargets(games []*game.Game, manifest *dll.Manifest) []dllMutationTarget {
	var targets []dllMutationTarget
	if manifest == nil {
		return targets
	}
	for _, entry := range games {
		for _, installed := range entry.DLLs {
			latest := manifest.GetLatestDLL(strings.ToLower(string(installed.Type)))
			if latest == nil || installed.Version != "" && !dll.IsNewer(installed.Version, latest.Version) {
				continue
			}
			targets = append(targets, newDLLMutationTarget(entry, installed, latest.Version))
		}
	}
	return targets
}

// dllMutationConfirmation is the single confirmation contract for every TUI
// operation that can write a game DLL.
type dllMutationConfirmation struct {
	title   string
	items   []string
	backup  string
	targets []dllMutationTarget

	bodyFocused     bool
	confirmSelected bool
	scrollOffset    int
	width           int
	height          int
}

func newDLLMutationConfirmation(title string, targets []dllMutationTarget, backup string) *dllMutationConfirmation {
	items := make([]string, 0, len(targets))
	for _, target := range targets {
		version := target.targetVersion
		if target.currentVersion != "" {
			version = target.currentVersion + " → " + target.targetVersion
		}
		items = append(items, fmt.Sprintf("%s · %s · %s\n%s", target.gameName, target.family, version, target.path))
	}
	return &dllMutationConfirmation{title: title, items: items, backup: backup, targets: targets}
}

func dllUpdateRequests(targets []dllMutationTarget, cachedOnly bool) []dll.UpdateRequest {
	requests := make([]dll.UpdateRequest, 0, len(targets))
	for _, target := range targets {
		requests = append(requests, dll.UpdateRequest{
			AppID:         target.appID,
			DLLType:       target.manifestKey,
			Version:       target.targetVersion,
			InstalledPath: target.path,
			CachedOnly:    cachedOnly,
		})
	}
	return requests
}

func dllCancellationResult(operation string) string {
	return operation + " cancelled, no operation ran"
}

func (confirmation *dllMutationConfirmation) update(key tea.KeyPressMsg) (confirm, cancel bool) {
	return confirmation.updateAction(dllOverlayAction(key))
}

// updateAction keeps the confirmation safe when its initiating control and
// the dialog both use Enter: Cancel is selected until the user chooses Confirm.
func (confirmation *dllMutationConfirmation) updateAction(action KeyAction) (confirm, cancel bool) {
	if confirmation == nil {
		return false, false
	}
	switch action {
	case ActionOverlayFocusNext:
		confirmation.bodyFocused = !confirmation.bodyFocused
	case ActionOverlayLeft, ActionOverlayRight:
		if !confirmation.bodyFocused {
			confirmation.confirmSelected = !confirmation.confirmSelected
		}
	case ActionOverlayPrevious:
		if confirmation.bodyFocused {
			confirmation.scrollOffset = max(confirmation.scrollOffset-1, 0)
		}
	case ActionOverlayNext:
		if confirmation.bodyFocused {
			confirmation.scrollOffset = min(confirmation.scrollOffset+1, confirmation.maximumScroll())
		}
	case ActionOverlayConfirm:
		if !confirmation.bodyFocused {
			return confirmation.confirmSelected, !confirmation.confirmSelected
		}
	case ActionOverlayClose:
		return false, true
	}
	return false, false
}

func (confirmation *dllMutationConfirmation) view(styles *Styles) string {
	return confirmation.viewSized(styles, 100, 30)
}

func (confirmation *dllMutationConfirmation) bodyLines() []string {
	width := confirmation.width
	if width <= 0 {
		width = 100
	}
	body := strings.Join(confirmation.items, "\n\n") + "\n\nBackup: " + confirmation.backup
	return strings.Split(lipgloss.NewStyle().Width(max(width, 1)).Render(body), "\n")
}

func (confirmation *dllMutationConfirmation) maximumScroll() int {
	height := confirmation.height
	if height <= 0 {
		height = 30
	}
	return max(len(confirmation.bodyLines())-max(height-5, 1), 0)
}

// viewSized reserves the buttons and local controls before allocating the
// scrollable target list. Long paths and backup policy remain inspectable.
func (confirmation *dllMutationConfirmation) viewSized(styles *Styles, width, height int) string {
	if confirmation == nil {
		return ""
	}
	confirmation.width, confirmation.height = max(width, 1), max(height, 6)
	lines := confirmation.bodyLines()
	capacity := max(confirmation.height-5, 1)
	confirmation.scrollOffset = min(confirmation.scrollOffset, confirmation.maximumScroll())
	var builder strings.Builder
	builder.WriteString(styles.Title.Render(truncate(confirmation.title, confirmation.width)))
	fmt.Fprintf(&builder, "\n%d DLL target(s)\n", len(confirmation.targets))
	for index := confirmation.scrollOffset; index < min(confirmation.scrollOffset+capacity, len(lines)); index++ {
		builder.WriteString(lines[index] + "\n")
	}
	for padding := len(lines) - confirmation.scrollOffset; padding < capacity; padding++ {
		builder.WriteByte('\n')
	}
	if confirmation.bodyFocused {
		builder.WriteString(styles.Dim.Render("Cancel   Confirm"))
		builder.WriteString("\n" + styles.FocusStyle().Render(confirmation.hint()))
	} else {
		cancelStyle, confirmStyle := styles.FocusStyle(), styles.Normal
		if confirmation.confirmSelected {
			cancelStyle, confirmStyle = confirmStyle, cancelStyle
		}
		builder.WriteString(cancelStyle.Render("[ Cancel ]") + "  " + confirmStyle.Render("[ Confirm ]"))
		builder.WriteString("\n" + styles.Dim.Render(confirmation.hint()))
	}
	return builder.String()
}

func (confirmation *dllMutationConfirmation) hint() string {
	if !confirmation.bodyFocused {
		return "←/→ Choose · Enter Activate · Tab Details"
	}
	if confirmation.maximumScroll() > 0 {
		return "↑/↓ Scroll · Tab Buttons"
	}
	return "Tab Buttons"
}

// dllOverlayAction is only an adapter for widgets used outside shell dispatch.
// It accepts the displayed basic controls and no former letter shortcuts.
func dllOverlayAction(key tea.KeyPressMsg) KeyAction {
	switch key.String() {
	case "tab":
		return ActionOverlayFocusNext
	case "left":
		return ActionOverlayLeft
	case "right":
		return ActionOverlayRight
	case "up":
		return ActionOverlayPrevious
	case "down":
		return ActionOverlayNext
	case "enter":
		return ActionOverlayConfirm
	}
	return ActionNoOp
}

func dllResultLines(message string, width int) []string {
	return strings.Split(lipgloss.NewStyle().Width(max(width, 20)).Render(message), "\n")
}

func dllResultMaximumScroll(message string, width, height int) int {
	return max(len(dllResultLines(message, width))-max(height-4, 1), 0)
}

func dllResultView(styles *Styles, title, message string, bodyFocused bool, offset, width, height int) string {
	height = max(height, 6)
	lines := dllResultLines(message, width)
	capacity := max(height-4, 1)
	offset = min(max(offset, 0), dllResultMaximumScroll(message, width, height))
	var builder strings.Builder
	builder.WriteString(styles.Title.Render(title) + "\n")
	for index := offset; index < min(offset+capacity, len(lines)); index++ {
		builder.WriteString(lines[index] + "\n")
	}
	for padding := len(lines) - offset; padding < capacity; padding++ {
		builder.WriteByte('\n')
	}
	if bodyFocused {
		builder.WriteString(styles.Normal.Render("[ Close ]"))
		builder.WriteString("\n" + styles.FocusStyle().Render(dllResultHint(message, true, width, height)))
	} else {
		builder.WriteString(styles.FocusStyle().Render("[ Close ]"))
		builder.WriteString("\n" + styles.Dim.Render(dllResultHint(message, false, width, height)))
	}
	return builder.String()
}

func dllResultHint(message string, bodyFocused bool, width, height int) string {
	if !bodyFocused {
		return "Enter Close · Tab Details"
	}
	if dllResultMaximumScroll(message, width, height) > 0 {
		return "↑/↓ Scroll · Tab Close"
	}
	return "Tab Close"
}
