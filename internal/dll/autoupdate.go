package dll

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/game"
)

// AutoUpdateResult reports what the startup DLL update did.
type AutoUpdateResult struct {
	Batch   BatchResult
	Running []string
	Err     error
}

// AutoUpdate brings every game's DLLs to the manifest's latest version without
// asking. Because nobody confirms it, it is stricter than a manual update: it
// leaves deny-listed games, running games, and DLLs of unknown version alone,
// and it only touches DLLs that are older than the manifest's latest. Each
// swap keeps the manual path's backup and checksum checks.
func AutoUpdate(running func([]*game.Game) map[uint64]bool) AutoUpdateResult {
	manifest, err := GetManifest(false, "")
	if err != nil {
		return AutoUpdateResult{Err: fmt.Errorf("load DLL manifest: %w", err)}
	}
	database, err := game.LoadDatabase()
	if err != nil {
		return AutoUpdateResult{Err: fmt.Errorf("load game database: %w", err)}
	}
	games := database.List()
	sort.Slice(games, func(left, right int) bool { return games[left].Name < games[right].Name })
	var candidates []*game.Game
	pending := make(map[uint64][]UpdateRequest)
	for _, entry := range games {
		if denied, _ := denylist.IsDenied(entry.AppID); denied {
			continue
		}
		if requests := autoUpdateRequests(entry, manifest); len(requests) > 0 {
			candidates = append(candidates, entry)
			pending[entry.AppID] = requests
		}
	}
	result := AutoUpdateResult{}
	if len(candidates) == 0 {
		return result
	}
	busy := running(candidates)
	var requests []UpdateRequest
	for _, entry := range candidates {
		if busy[entry.AppID] {
			result.Running = append(result.Running, entry.Name)
			continue
		}
		requests = append(requests, pending[entry.AppID]...)
	}
	if len(requests) > 0 {
		result.Batch = BatchUpdate(requests, nil)
	}
	return result
}

func autoUpdateRequests(entry *game.Game, manifest *Manifest) []UpdateRequest {
	var requests []UpdateRequest
	for _, detected := range entry.DLLs {
		dllType := strings.ToLower(string(detected.Type))
		latest := manifest.GetLatestDLL(dllType)
		if detected.Version == "" || latest == nil || !IsNewer(detected.Version, latest.Version) {
			continue
		}
		requests = append(requests, UpdateRequest{AppID: entry.AppID, DLLType: dllType, Version: latest.Version, InstalledPath: detected.Path})
	}
	return requests
}

// Notice is a one-line summary for the TUI and GUI. It is empty when every DLL
// was already current, so a quiet start stays quiet.
func (result AutoUpdateResult) Notice() string {
	if result.Err != nil {
		return "DLL auto-update skipped: " + result.Err.Error()
	}
	var parts []string
	if updated := result.Batch.Updated; updated > 0 {
		parts = append(parts, fmt.Sprintf("updated %d %s", updated, plural(updated, "DLL", "DLLs")))
	}
	if failed := result.Batch.Failed; failed > 0 {
		for _, item := range result.Batch.Items {
			if item.Err != nil {
				parts = append(parts, fmt.Sprintf("%d failed (%v)", failed, item.Err))
				break
			}
		}
	}
	if len(result.Running) > 0 {
		parts = append(parts, "skipped "+strings.Join(result.Running, ", ")+" while running")
	}
	if len(parts) == 0 {
		return ""
	}
	return "DLL auto-update: " + strings.Join(parts, "; ")
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
