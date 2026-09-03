package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
		family:         strings.ToUpper(string(installed.Type)),
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
}

func newDLLMutationConfirmation(title string, targets []dllMutationTarget, backup string) *dllMutationConfirmation {
	items := make([]string, 0, len(targets))
	for _, target := range targets {
		version := target.targetVersion
		if target.currentVersion != "" {
			version = target.currentVersion + " → " + target.targetVersion
		}
		items = append(items, fmt.Sprintf("%s — %s — %s — %s", target.gameName, target.family, target.path, version))
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
	return operation + " cancelled — no operation ran"
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
	builder.WriteString("\n")
	for _, item := range confirmation.items {
		builder.WriteString("• " + item + "\n")
	}
	builder.WriteString("\nBackup: " + confirmation.backup)
	builder.WriteString("\n\nEnter/Y confirm • Esc/q cancel")
	return builder.String()
}
