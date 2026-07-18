package dll

import (
	"fmt"
	"time"

	"github.com/jgabor/spela/internal/game"
)

// RefreshGameAfterMutation records the observable result of a completed DLL
// file mutation. A scan or persistence failure is returned explicitly because
// the files may already have changed and callers must report a partial outcome.
func RefreshGameAfterMutation(
	entry *game.Game,
	operation string,
	scan func(string) ([]game.DetectedDLL, error),
	save func() error,
) error {
	detected, err := scan(entry.InstallDir)
	if err != nil {
		return fmt.Errorf("scan install directory: %w", err)
	}
	entry.DLLs = detected
	entry.ScannedAt = time.Now()
	if err := save(); err != nil {
		return fmt.Errorf("save game database after %s: %w", operation, err)
	}
	return nil
}
