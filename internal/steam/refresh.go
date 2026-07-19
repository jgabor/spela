package steam

import (
	"fmt"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

// Rescan discovers installed games from Steam libraries and persists the database.
func Rescan(cfg *config.Config) (*game.Database, error) {
	return rescan(cfg, ScanLibraries)
}

func rescan(cfg *config.Config, scan func(string, []string) (*game.Database, error)) (*game.Database, error) {
	steamPath, additional := scanPaths(cfg)
	return game.Transaction(func(current *game.Database) (bool, error) {
		discovered, err := scan(steamPath, additional)
		if err != nil {
			return false, err
		}
		if discovered == nil {
			return false, fmt.Errorf("could not find Steam installation")
		}
		*current = *discovered
		return true, nil
	})
}

// RefreshIfNeeded rescans when the database is empty or rescan-on-startup is enabled.
func RefreshIfNeeded(existing *game.Database, cfg *config.Config) (*game.Database, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	if len(existing.Games) > 0 && !cfg.RescanOnStartup {
		return existing, nil
	}
	return Rescan(cfg)
}

func scanPaths(cfg *config.Config) (string, []string) {
	if cfg == nil {
		return ResolveSteamPath(""), nil
	}
	return ResolveSteamPath(cfg.SteamPath), cfg.AdditionalLibraryPaths
}
