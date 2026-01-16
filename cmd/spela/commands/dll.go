package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

var DLLCmd = &cobra.Command{
	Use:   "dll",
	Short: "Manage game DLLs",
	Long:  "List, update, and restore DLSS/FSR/XeSS DLLs for games.",
}

var dllListCmd = &cobra.Command{
	Use:   "list [game]",
	Short: "List detected DLLs",
	RunE:  runDLLList,
}

var dllCheckCmd = &cobra.Command{
	Use:   "check-updates",
	Short: "Check for DLL updates",
	RunE:  runDLLCheckUpdates,
}

var dllUpdateCmd = &cobra.Command{
	Use:   "update <game> <dll-type>",
	Short: "Update a DLL to the latest version",
	Long:  "Download and install the latest version of a DLL for a game.",
	Args:  cobra.ExactArgs(2),
	RunE:  runDLLUpdate,
}

var dllRestoreCmd = &cobra.Command{
	Use:   "restore <game>",
	Short: "Restore original DLLs from backup",
	Args:  cobra.ExactArgs(1),
	RunE:  runDLLRestore,
}

func init() {
	DLLCmd.AddCommand(dllListCmd)
	DLLCmd.AddCommand(dllCheckCmd)
	DLLCmd.AddCommand(dllUpdateCmd)
	DLLCmd.AddCommand(dllRestoreCmd)
}

func runDLLList(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var games []*game.Game

	if len(args) > 0 {
		var g *game.Game
		if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
			g = db.GetGame(appID)
		} else {
			g = db.GetGameByName(args[0])
		}
		if g == nil {
			return fmt.Errorf("game not found: %s", args[0])
		}
		games = []*game.Game{g}
	} else {
		games = db.GamesWithDLSS()
	}

	if len(games) == 0 {
		fmt.Println("No games with DLSS/FSR/XeSS DLLs found.")
		return nil
	}

	for _, g := range games {
		fmt.Printf("%s (%d)\n", g.Name, g.AppID)
		for _, d := range g.DLLs {
			version := d.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Printf("  %s: %s\n", d.Name, version)
			fmt.Printf("    %s\n", d.Path)
		}
	}

	return nil
}

func runDLLCheckUpdates(cmd *cobra.Command, _ []string) error {
	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	games := db.GamesWithDLSS()
	if len(games) == 0 {
		fmt.Println("No games with DLSS/FSR/XeSS DLLs found.")
		return nil
	}

	hasUpdates := false
	for _, g := range games {
		gameHasUpdates := false
		var updates []string

		for _, d := range g.DLLs {
			dllType := strings.ToLower(string(d.Type))
			latest := manifest.GetLatestDLL(dllType)
			if latest == nil {
				continue
			}

			if d.Version == "" || dll.IsNewer(d.Version, latest.Version) {
				gameHasUpdates = true
				hasUpdates = true
				current := d.Version
				if current == "" {
					current = "unknown"
				}
				updates = append(updates, fmt.Sprintf("  %s: %s -> %s", d.Name, current, latest.Version))
			}
		}

		if gameHasUpdates {
			fmt.Printf("%s (%d)\n", g.Name, g.AppID)
			for _, u := range updates {
				fmt.Println(u)
			}
		}
	}

	if !hasUpdates {
		fmt.Println("All DLLs are up to date.")
	}

	return nil
}

func runDLLUpdate(cmd *cobra.Command, args []string) error {
	gameArg := args[0]
	dllType := strings.ToLower(args[1])

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(gameArg, 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(gameArg)
	}
	if g == nil {
		return fmt.Errorf("game not found: %s", gameArg)
	}

	var targetDLL *game.DetectedDLL
	for i := range g.DLLs {
		if strings.ToLower(string(g.DLLs[i].Type)) == dllType {
			targetDLL = &g.DLLs[i]
			break
		}
	}
	if targetDLL == nil {
		return fmt.Errorf("game does not have a %s DLL", dllType)
	}

	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	latest := manifest.GetLatestDLL(dllType)
	if latest == nil {
		return fmt.Errorf("no %s versions available in manifest", dllType)
	}

	if targetDLL.Version != "" && !dll.IsNewer(targetDLL.Version, latest.Version) {
		fmt.Printf("%s is already at the latest version (%s)\n", targetDLL.Name, targetDLL.Version)
		return nil
	}

	fmt.Printf("Downloading %s %s...\n", dllType, latest.Version)

	cachePath, err := dll.DownloadDLLWithProgress(latest, dllType, func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		} else {
			fmt.Printf("\rDownloading: %d bytes", downloaded)
		}
	})
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to download DLL: %w", err)
	}

	var gameDLLs []dll.GameDLL
	for _, d := range g.DLLs {
		gameDLLs = append(gameDLLs, dll.GameDLL{
			Name:    d.Name,
			Path:    d.Path,
			Version: d.Version,
		})
	}

	if err := dll.SwapDLL(g.AppID, g.Name, gameDLLs, targetDLL.Name, cachePath); err != nil {
		return fmt.Errorf("failed to swap DLL: %w", err)
	}

	fmt.Printf("Updated %s to version %s\n", targetDLL.Name, latest.Version)
	return nil
}

func runDLLRestore(cmd *cobra.Command, args []string) error {
	gameArg := args[0]

	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(gameArg, 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(gameArg)
	}
	if g == nil {
		return fmt.Errorf("game not found: %s", gameArg)
	}

	if !dll.BackupExists(g.AppID) {
		return fmt.Errorf("no backup found for %s", g.Name)
	}

	if err := dll.RestoreBackup(g.AppID); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	fmt.Printf("Restored original DLLs for %s\n", g.Name)
	return nil
}
