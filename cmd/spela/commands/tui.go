package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/tui"
)

var TUICmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI",
	Long:  "Launch the interactive terminal user interface for browsing games and managing profiles.",
	RunE:  runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
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
