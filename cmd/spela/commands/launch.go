package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/overlay"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/xdg"
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

	restore := profile.NewRestorePoint()
	restore.SaveAllProfileEnvVars()

	e := env.New()

	var cleanups []func()
	if p != nil {
		cleanups = p.Apply(e)
	}

	l := launcher.New(g)
	l.Environment = e

	l.OnCleanup(restore.Restore)
	for _, cleanup := range cleanups {
		l.OnCleanup(cleanup)
	}

	if p != nil && p.Overlay.Enabled {
		ipc, overlayCleanup, err := startOverlayCollector(p, g.AppID)
		if err != nil {
			log.Printf("Warning: overlay collector failed to start: %v", err)
		} else {
			e.Set("SPELA_OVERLAY_IPC", ipc.Path())
			l.OnCleanup(overlayCleanup)
		}
	}

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

func startOverlayCollector(p *profile.Profile, appID uint64) (*overlay.IPCFile, func(), error) {
	runtimeDir := xdg.RuntimeDir()
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create runtime dir: %w", err)
	}

	ipcPath := filepath.Join(runtimeDir, fmt.Sprintf("overlay-%d.dat", appID))
	ipc, err := overlay.CreateIPC(ipcPath)
	if err != nil {
		return nil, nil, fmt.Errorf("create ipc: %w", err)
	}

	position := overlay.ParsePosition(p.Overlay.Position)
	thresholds := overlay.DefaultThresholds()

	collect := func() overlay.SharedState {
		state := overlay.SharedState{
			Visible:  true,
			Position: position,
		}

		metrics, err := gpu.GetGPUMetrics()
		if err != nil {
			return state
		}

		state.Temperature = metrics.Temperature
		state.PowerDraw = metrics.PowerDraw
		state.PowerLimit = metrics.PowerLimit
		state.Utilization = metrics.Utilization
		state.VRAMUsedMB = metrics.MemoryUsed
		state.VRAMTotalMB = metrics.MemoryTotal
		state.GraphicsMHz = metrics.GraphicsClock
		state.MemoryMHz = metrics.MemoryClock
		state.FanSpeed = metrics.FanSpeed

		input := overlay.AlertInput{
			Temperature:   metrics.Temperature,
			PowerDraw:     metrics.PowerDraw,
			PowerLimit:    metrics.PowerLimit,
			GraphicsClock: metrics.GraphicsClock,
			FanSpeed:      metrics.FanSpeed,
		}
		if metrics.ThrottleReasons != nil {
			input.ThrottleThermal = metrics.ThrottleReasons.ThermalHardware || metrics.ThrottleReasons.ThermalSoftware
			input.ThrottlePower = metrics.ThrottleReasons.PowerCap || metrics.ThrottleReasons.PowerBrake
		}

		alerts := overlay.Evaluate(input, thresholds)
		if len(alerts) > 0 {
			state.AlertActive = true
			highest := overlay.AlertInfo
			for _, a := range alerts {
				if a.Severity > highest {
					highest = a.Severity
				}
			}
			state.AlertSeverity = highest
		}

		return state
	}

	c := overlay.NewCollector(ipc, collect, 500*time.Millisecond)
	c.Start()

	cleanup := func() {
		c.Stop()
		_ = ipc.Close()
		_ = os.Remove(ipcPath)
	}

	return ipc, cleanup, nil
}
