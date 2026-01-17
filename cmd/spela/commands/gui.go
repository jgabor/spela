package commands

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gui"
	"github.com/jgabor/spela/internal/lock"
	"github.com/jgabor/spela/internal/tui"
)

var GUICmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch graphical user interface",
	Long:  "Launch the graphical user interface for browsing games and managing profiles.",
	RunE:  runGUI,
}

func runGUI(cmd *cobra.Command, args []string) error {
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	if !hasDisplay() {
		return fallbackToTUI("no display available")
	}

	if err := gui.Run(); err != nil {
		return fallbackToTUI(err.Error())
	}
	return nil
}

func hasDisplay() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func fallbackToTUI(reason string) error {
	slog.Debug("GUI unavailable, falling back to TUI", "reason", reason)

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	if len(db.Games) == 0 {
		fmt.Println("No games found. Run 'spela scan' first.")
		return nil
	}

	return tui.Run(db)
}
