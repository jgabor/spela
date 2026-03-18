package commands

import (
	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/gui"
	"github.com/jgabor/spela/internal/lock"
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

	return gui.Run()
}
