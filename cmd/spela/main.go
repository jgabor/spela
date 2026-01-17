package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/cmd/spela/commands"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "spela [command]",
	Short:   "Linux gaming optimization tool",
	Long:    "Spela is a Linux gaming optimization tool that combines DLSS/DLL management with comprehensive gaming environment setup.",
	Version: version,
	RunE:    runRoot,
}

func init() {
	rootCmd.AddCommand(commands.ScanCmd)
	rootCmd.AddCommand(commands.ListCmd)
	rootCmd.AddCommand(commands.ShowCmd)
	rootCmd.AddCommand(commands.LaunchCmd)
	rootCmd.AddCommand(commands.ProfileCmd)
	rootCmd.AddCommand(commands.ConfigCmd)
	rootCmd.AddCommand(commands.DLLCmd)
	rootCmd.AddCommand(commands.DLSSCmd)
	rootCmd.AddCommand(commands.GPUCmd)
	rootCmd.AddCommand(commands.CPUCmd)
	rootCmd.AddCommand(commands.TUICmd)
	rootCmd.AddCommand(commands.GUICmd)
	rootCmd.AddCommand(commands.DenylistCmd)
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	if launcher.IsWrapperMode(args) {
		return runWrapperMode(args)
	}

	return cmd.Help()
}

func runWrapperMode(args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	g := launcher.DetectGameFromCommand(db, args)

	l := launcher.New(g)
	return l.Launch(args)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
