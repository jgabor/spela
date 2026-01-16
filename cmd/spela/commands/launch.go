package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
)

var launchGameID uint64

var LaunchCmd = &cobra.Command{
	Use:   "launch <game>",
	Short: "Launch a game with its profile",
	Long:  "Launch a game applying its profile settings. Can specify game by name or ID.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLaunch,
}

func init() {
	LaunchCmd.Flags().Uint64Var(&launchGameID, "game-id", 0, "Launch by Steam App ID")
}

func runLaunch(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var g *game.Game

	if launchGameID != 0 {
		g = db.GetGame(launchGameID)
	} else if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found")
	}

	p, err := profile.LoadOrDefault(g.AppID, profile.Preset(cfg.DefaultPreset))
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	restore := profile.NewRestorePoint()
	restore.SaveAllProfileEnvVars()

	e := env.New()
	cleanups := p.Apply(e)

	l := launcher.New(g)
	l.Environment = e

	l.OnCleanup(restore.Restore)
	for _, cleanup := range cleanups {
		l.OnCleanup(cleanup)
	}

	launchArgs := args
	if launchGameID != 0 || len(args) == 1 {
		launchArgs = []string{"steam", fmt.Sprintf("steam://rungameid/%d", g.AppID)}
	}

	fmt.Printf("Launching %s with %s profile...\n", g.Name, p.Preset)
	return l.Launch(launchArgs)
}
