package steam

import (
	"fmt"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

// Rescan discovers installed games from Steam libraries and persists the database.
func Rescan(cfg *config.Config) (*game.Database, error) {
	steamPath, additional := scanPaths(cfg)
	db, err := ScanLibraries(steamPath, additional)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, fmt.Errorf("could not find Steam installation")
	}
	if err := db.Save(); err != nil {
		return nil, err
	}
	return db, nil
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
