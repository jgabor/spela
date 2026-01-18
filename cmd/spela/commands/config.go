package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/config"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global configuration",
	Long:  "View and modify global spela configuration settings.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	ConfigCmd.AddCommand(configShowCmd)
	ConfigCmd.AddCommand(configSetCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Print(string(data))
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	key, value := args[0], args[1]

	switch key {
	case "log_level":
		cfg.LogLevel = config.LogLevel(value)
	case "shader_cache":
		cfg.ShaderCache = value
	case "check_updates":
		cfg.CheckUpdates = value == "true" || value == "1"
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}
