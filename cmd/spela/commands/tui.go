package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/lock"
	"github.com/jgabor/spela/internal/steam"
	"github.com/jgabor/spela/internal/tui"
	"github.com/jgabor/spela/internal/xdg"
)

var TUICmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI",
	Long:  "Launch the interactive terminal user interface for browsing games and managing profiles.",
	RunE:  runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Cannot load Spela configuration %s: %v\nFix the YAML or remove the file to restore defaults.\n", xdg.ConfigPath("config.yaml"), err)
		return nil
	}
	db, err = steam.RefreshIfNeeded(db, cfg)
	if err != nil {
		return fmt.Errorf("failed to refresh game database: %w", err)
	}
	return tui.Run(db)
}
