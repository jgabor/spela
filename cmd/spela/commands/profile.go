package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage game profiles",
	Long:  "Create, edit, and manage per-game configuration profiles.",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <game>",
	Short: "Create a profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileCreate,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileShow,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <game>",
	Short: "Delete a game's profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

func init() {
	ProfileCmd.AddCommand(profileListCmd)
	ProfileCmd.AddCommand(profileCreateCmd)
	ProfileCmd.AddCommand(profileShowCmd)
	ProfileCmd.AddCommand(profileDeleteCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	profiles, err := profile.List()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found.")
		return nil
	}

	db, _ := game.LoadDatabase()

	for appID, p := range profiles {
		name := fmt.Sprintf("%d", appID)
		if db != nil {
			if g := db.GetGame(appID); g != nil {
				name = g.Name
			}
		}
		profileName := p.Name
		if profileName == "" {
			profileName = "custom"
		}
		fmt.Printf("%s %s: %s\n", tui.CLIPrimary(name), tui.CLIDim(fmt.Sprintf("(%d)", appID)), tui.CLISecondary(profileName))
	}

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if profile.Exists(g.AppID) {
		return fmt.Errorf("profile already exists for %s", g.Name)
	}

	p := &profile.Profile{Name: g.Name}

	if err := profile.Save(g.AppID, p); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("%s %s\n", tui.CLISuccess("Created profile for"), tui.CLIPrimary(g.Name))
	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("no profile for %s", g.Name)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return fmt.Errorf("failed to load game database: %w", err)
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if err := profile.Delete(g.AppID); err != nil {
		return err
	}

	fmt.Printf("%s %s\n", tui.CLISuccess("Deleted profile for"), tui.CLIPrimary(g.Name))
	return nil
}
