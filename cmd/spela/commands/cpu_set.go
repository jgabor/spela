package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

var (
	cpuSetGovernor string
	cpuSetSMT      string
)

var cpuSetCmd = &cobra.Command{
	Use:   "set <game>",
	Short: "Set CPU profile for a game",
	Long:  "Set CPU profile settings for a game. These are applied at game launch, unlike 'cpu governor' and 'cpu smt' which change live system state.",
	Args:  cobra.ExactArgs(1),
	RunE:  runCPUSet,
}

var cpuShowCmd = &cobra.Command{
	Use:   "show <game>",
	Short: "Show CPU profile for a game",
	Args:  cobra.ExactArgs(1),
	RunE:  runCPUShow,
}

func init() {
	cpuSetCmd.Flags().StringVar(&cpuSetGovernor, "governor", "", "CPU frequency governor (performance, powersave, schedutil, ondemand)")
	cpuSetCmd.Flags().StringVar(&cpuSetSMT, "smt", "", "SMT control (on, off, default)")

	CPUCmd.AddCommand(cpuSetCmd)
	CPUCmd.AddCommand(cpuShowCmd)
}

func runCPUSet(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		p = &profile.Profile{Name: g.Name}
	}

	changed := false

	if cpuSetGovernor != "" {
		if cpuSetGovernor == "default" {
			p.CPU.Governor = ""
		} else {
			p.CPU.Governor = cpuSetGovernor
		}
		changed = true
	}

	if cpuSetSMT != "" {
		switch cpuSetSMT {
		case "on", "true":
			b := true
			p.CPU.SMT = &b
		case "off", "false":
			b := false
			p.CPU.SMT = &b
		case "default":
			p.CPU.SMT = nil
		default:
			return fmt.Errorf("invalid SMT value: %s (use on, off, or default)", cpuSetSMT)
		}
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := profile.Save(g.AppID, p); err != nil {
		return err
	}

	fmt.Printf("Updated CPU profile for %s\n", g.Name)
	return nil
}

func runCPUShow(cmd *cobra.Command, args []string) error {
	db, err := game.LoadDatabase()
	if err != nil {
		return err
	}

	g := db.FindGame(args[0])
	if g == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}

	p, err := profile.Load(g.AppID)
	if err != nil {
		return err
	}
	if p == nil {
		fmt.Printf("No profile for %s\n", g.Name)
		p = &profile.Profile{}
	}

	fmt.Printf("CPU profile for %s:\n\n", g.Name)
	fmt.Printf("%s  %s\n", tui.CLIDim("Governor:"), displayGPUString(p.CPU.Governor))
	fmt.Printf("%s  %s\n", tui.CLIDim("SMT:"), displaySMT(p.CPU.SMT))
	fmt.Printf("%s  %s\n", tui.CLIDim("Affinity:"), displayGPUString(p.CPU.Affinity))

	return nil
}

func displaySMT(b *bool) string {
	if b == nil {
		return "(default)"
	}
	if *b {
		return "on"
	}
	return "off"
}
