package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

var cpuShowJSON bool

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

	cpuShowCmd.Flags().BoolVar(&cpuShowJSON, "json", false, "Output as JSON")

	CPUCmd.AddCommand(cpuSetCmd)
	CPUCmd.AddCommand(cpuShowCmd)
	CPUCmd.AddCommand(cpuProfileResetCmd)
}

var cpuProfileResetCmd = &cobra.Command{
	Use:   "profile-reset <game> <field>",
	Short: "Reset a CPU profile field to inherit from defaults",
	Long: `Reset a CPU profile field back to inherited. Valid fields:
  governor, smt, affinity.`,
	Args: cobra.ExactArgs(2),
	RunE: runCPUProfileReset,
}

func runCPUProfileReset(_ *cobra.Command, args []string) error {
	return resetProfileField(args, "cpu", "CPU", "")
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

	changes := profileChanges{name: g.Name}

	if cpuSetGovernor != "" {
		if err := validateCPUGovernorFlag("governor", cpuSetGovernor); err != nil {
			return err
		}
		if cpuSetGovernor == "default" {
			changes.reset(profile.FieldCPUGovernor)
		} else {
			changes.set(profile.FieldCPUGovernor, cpuSetGovernor)
		}
	}

	if cpuSetSMT != "" {
		switch cpuSetSMT {
		case "on", "true":
			changes.set(profile.FieldCPUSMT, true)
		case "off", "false":
			changes.set(profile.FieldCPUSMT, false)
		case "default":
			changes.reset(profile.FieldCPUSMT)
		default:
			return fmt.Errorf("invalid SMT value: %s (use on, off, or default)", cpuSetSMT)
		}
	}

	if len(changes.mutations) == 0 {
		fmt.Println("No changes specified. Use --help to see available options.")
		return nil
	}

	if err := changes.save(g.AppID); err != nil {
		return err
	}

	fmt.Printf("Updated CPU profile for %s\n", g.Name)
	return nil
}

func runCPUShow(cmd *cobra.Command, args []string) error {
	g, p, resolved, err := resolvedProfileForShow(args[0])
	if err != nil {
		return err
	}

	if cpuShowJSON {
		data, err := json.MarshalIndent(resolved.CPU, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("CPU profile for %s:\n\n", g.Name)
	fmt.Println(renderField("Governor:", profile.FieldCPUGovernor, p, displayGPUString(resolved.CPU.Governor)))
	fmt.Println(renderField("SMT:", profile.FieldCPUSMT, p, displaySMT(resolved.CPU.SMT)))
	fmt.Println(renderField("Affinity:", profile.FieldCPUAffinity, p, displayGPUString(resolved.CPU.Affinity)))

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
