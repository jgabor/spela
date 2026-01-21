//go:build !wails
// +build !wails

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/cmd/spela/commands"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
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

	invocation := launcher.ParseWrapperArguments(args)
	if len(invocation.Command) == 0 {
		return fmt.Errorf("no command provided for wrapper mode")
	}

	g := launcher.DetectGameFromCommand(db, args)
	if g == nil {
		fmt.Fprintln(os.Stderr, "Warning: could not detect game for wrapper invocation")
	}

	var p *profile.Profile
	if g != nil {
		p, err = profile.Load(g.AppID)
		if err != nil {
			return fmt.Errorf("failed to load profile: %w", err)
		}
	}

	restore := profile.NewRestorePoint()
	restore.SaveAllProfileEnvVars()

	e := env.New()
	var cleanups []func()
	if p != nil {
		cleanups = p.Apply(e)
	}
	for key, value := range invocation.Environment {
		e.Set(key, value)
	}

	l := launcher.New(g)
	l.Profile = p
	l.Environment = e

	l.OnCleanup(restore.Restore)
	for _, cleanup := range cleanups {
		l.OnCleanup(cleanup)
	}

	if p != nil {
		fmt.Printf("Launching %s with profile...\n", g.Name)
	} else if g != nil {
		fmt.Printf("Launching %s (no profile)...\n", g.Name)
	} else {
		fmt.Printf("Launching command...\n")
	}
	return l.Launch(invocation.Command)
}

func main() {
	args := os.Args[1:]
	if launcher.IsWrapperMode(args) {
		if err := runWrapperMode(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
