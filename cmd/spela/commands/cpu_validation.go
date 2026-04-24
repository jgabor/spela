package commands

import (
	"fmt"

	"github.com/jgabor/spela/internal/cpu"
)

var validateCPUGovernorAvailable = cpu.ValidateGovernorAvailable

func validateCPUGovernorFlag(flagName, governor string) error {
	if governor == "" || governor == "default" {
		return nil
	}
	if err := validateCPUGovernorAvailable(cpu.Governor(governor)); err != nil {
		return fmt.Errorf("--%s: %w", flagName, err)
	}
	return nil
}
