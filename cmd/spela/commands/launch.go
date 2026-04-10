package commands

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var (
	launchGameID uint64
	launchDryRun bool
)

var LaunchCmd = &cobra.Command{
	Use:   "launch <game>",
	Short: "Launch a game with its profile",
	Long:  "Launch a game applying its profile settings. Can specify game by name or ID.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLaunch,
}

func init() {
	LaunchCmd.Flags().Uint64Var(&launchGameID, "game-id", 0, "Launch by Steam App ID")
	LaunchCmd.Flags().BoolVar(&launchDryRun, "dry-run", false, "Show what would happen without launching")
}

func runLaunch(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game

	if launchGameID != 0 {
		g = db.GetGame(launchGameID)
	} else {
		g = db.FindGame(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found")
	}

	p, err := profile.LoadEffective(g.AppID)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	if launchDryRun {
		return runLaunchDryRun(g, p, args)
	}

	l := launcher.New(g)
	l.Profile = p
	l.Prepare()

	launchArgs := args
	if launchGameID != 0 || len(args) == 1 {
		launchArgs = []string{"steam", fmt.Sprintf("steam://rungameid/%d", g.AppID)}
	}

	if p != nil {
		fmt.Printf("Launching %s with profile...\n", g.Name)
	} else {
		fmt.Printf("Launching %s (no profile)...\n", g.Name)
	}
	return l.Launch(launchArgs)
}

func runLaunchDryRun(g *game.Game, p *profile.Profile, args []string) error {
	fmt.Printf("%s %s (AppID %d)\n\n", tui.CLIDim("Game:"), tui.CLIPrimary(g.Name), g.AppID)

	launchCmd := fmt.Sprintf("steam steam://rungameid/%d", g.AppID)
	if len(args) > 1 {
		launchCmd = fmt.Sprintf("%s", args)
	}
	fmt.Printf("%s %s\n", tui.CLIDim("Command:"), launchCmd)

	if p == nil {
		fmt.Printf("\n%s\n", tui.CLIDim("No profile — default settings"))
		return nil
	}

	// Environment variables
	e := env.New()
	p.ApplyEnv(e)
	envVars := e.All()
	if len(envVars) > 0 {
		fmt.Printf("\n%s\n", tui.CLIPrimary("Environment variables"))
		keys := make([]string, 0, len(envVars))
		for k := range envVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s=%s\n", tui.CLIDim(k), envVars[k])
		}
	}

	// Hardware settings (privileged, via pkexec)
	hasHW := p.GPU.ClockOffset != 0 || p.GPU.MemoryOffset != 0 || p.GPU.PowerLimit > 0 || p.GPU.FanSpeed > 0 || p.CPU.Governor != "" || p.CPU.SMT != nil

	if hasHW {
		fmt.Printf("\n%s\n", tui.CLIPrimary("Hardware settings (via pkexec)"))
		if p.GPU.ClockOffset != 0 {
			fmt.Printf("  %s  %d MHz\n", tui.CLIDim("GPU clock offset:"), p.GPU.ClockOffset)
		}
		if p.GPU.MemoryOffset != 0 {
			fmt.Printf("  %s  %d MHz\n", tui.CLIDim("GPU memory offset:"), p.GPU.MemoryOffset)
		}
		if p.GPU.PowerLimit > 0 {
			fmt.Printf("  %s  %d W\n", tui.CLIDim("GPU power limit:"), p.GPU.PowerLimit)
		}
		if p.GPU.FanSpeed > 0 {
			fmt.Printf("  %s  %d%%\n", tui.CLIDim("GPU fan speed:"), p.GPU.FanSpeed)
		}
		if p.CPU.Governor != "" {
			fmt.Printf("  %s  %s\n", tui.CLIDim("CPU governor:"), p.CPU.Governor)
		}
		if p.CPU.SMT != nil {
			fmt.Printf("  %s  %s\n", tui.CLIDim("CPU SMT:"), strconv.FormatBool(*p.CPU.SMT))
		}
	}

	// Overlay
	if p.Overlay.Enabled {
		fmt.Printf("\n%s\n", tui.CLIPrimary("Overlay"))
		fmt.Printf("  %s  %s\n", tui.CLIDim("Position:"), profileVal(p.Overlay.Position))
		fmt.Printf("  %s  IPC file in XDG_RUNTIME_DIR\n", tui.CLIDim("Collector:"))
	}

	return nil
}
