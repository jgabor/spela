package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/game"
)

var DenylistCmd = &cobra.Command{
	Use:   "denylist",
	Short: "Manage the DLL swap deny list",
	Long:  "View and manage games that should not have their DLLs swapped (usually due to anti-cheat).",
}

var denylistShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the deny list",
	RunE:  runDenylistShow,
}

var denylistCheckCmd = &cobra.Command{
	Use:   "check <game>",
	Short: "Check if a game is denied",
	Args:  cobra.ExactArgs(1),
	RunE:  runDenylistCheck,
}

var denylistAllowCmd = &cobra.Command{
	Use:   "allow <game>",
	Short: "Force-allow a denied game",
	Args:  cobra.ExactArgs(1),
	RunE:  runDenylistAllow,
}

var (
	denylistDenyReason string
	denylistDenyCmd    = &cobra.Command{
		Use:   "deny <game>",
		Short: "Add a game to the deny list",
		Args:  cobra.ExactArgs(1),
		RunE:  runDenylistDeny,
	}
)

func init() {
	denylistDenyCmd.Flags().StringVar(&denylistDenyReason, "reason", "", "Reason for denying")

	DenylistCmd.AddCommand(denylistShowCmd)
	DenylistCmd.AddCommand(denylistCheckCmd)
	DenylistCmd.AddCommand(denylistAllowCmd)
	DenylistCmd.AddCommand(denylistDenyCmd)
}

func runDenylistShow(cmd *cobra.Command, args []string) error {
	list, err := denylist.LoadDenyList()
	if err != nil {
		return err
	}

	overrides, _ := denylist.LoadOverrides()

	fmt.Println("Denied games (anti-cheat):")
	for _, e := range list.Entries {
		allowed := false
		if overrides != nil {
			for _, id := range overrides.Allowed {
				if id == e.AppID {
					allowed = true
					break
				}
			}
		}

		status := ""
		if allowed {
			status = " [FORCE-ALLOWED]"
		}
		fmt.Printf("  %s (%d): %s%s\n", e.Name, e.AppID, e.Reason, status)
	}

	if overrides != nil && len(overrides.Denied) > 0 {
		fmt.Println("\nUser-denied games:")
		for _, e := range overrides.Denied {
			fmt.Printf("  %s (%d): %s\n", e.Name, e.AppID, e.Reason)
		}
	}

	return nil
}

func runDenylistCheck(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	var appID uint64
	var name string
	if g != nil {
		appID = g.AppID
		name = g.Name
	} else if id, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		appID = id
		name = args[0]
	} else {
		return fmt.Errorf("game not found: %s", args[0])
	}

	denied, reason := denylist.IsDenied(appID)
	if denied {
		fmt.Printf("%s is DENIED: %s\n", name, reason)
		fmt.Println("Use 'spela denylist allow' to force-allow (not recommended)")
	} else {
		fmt.Printf("%s is allowed for DLL swapping\n", name)
	}

	return nil
}

func runDenylistAllow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	var appID uint64
	var name string
	if g != nil {
		appID = g.AppID
		name = g.Name
	} else if id, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		appID = id
		name = args[0]
	} else {
		return fmt.Errorf("game not found: %s", args[0])
	}

	if err := denylist.Allow(appID); err != nil {
		return err
	}

	fmt.Printf("Force-allowed %s (%d)\n", name, appID)
	fmt.Println("WARNING: Swapping DLLs in anti-cheat games may result in bans!")
	return nil
}

func runDenylistDeny(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	var g *game.Game
	if appID, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		g = db.GetGame(appID)
	} else {
		g = db.GetGameByName(args[0])
	}

	var appID uint64
	var name string
	if g != nil {
		appID = g.AppID
		name = g.Name
	} else if id, err := strconv.ParseUint(args[0], 10, 64); err == nil {
		appID = id
		name = args[0]
	} else {
		return fmt.Errorf("game not found: %s", args[0])
	}

	reason := denylistDenyReason
	if reason == "" {
		reason = "user-specified"
	}

	if err := denylist.Deny(appID, name, reason); err != nil {
		return err
	}

	fmt.Printf("Denied %s (%d): %s\n", name, appID, reason)
	return nil
}
