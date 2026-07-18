package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/cpu"
)

func TestApplyProfileSupportedApplyAndResetBatches(t *testing.T) {
	var calls []string
	operations := applyProfileOperations{
		lockGraphicsClocks: func(int) error { calls = append(calls, "graphics"); return nil },
		lockMemoryClocks:   func(int) error { calls = append(calls, "memory"); return nil },
		setPowerLimit:      func(int) error { calls = append(calls, "power"); return nil },
		setFanSpeed:        func(int) error { calls = append(calls, "fan"); return nil },
		resetClocks:        func() error { calls = append(calls, "reset clocks"); return nil },
		resetFanSpeed:      func() error { calls = append(calls, "reset fan"); return nil },
		setGovernor:        func(cpu.Governor) error { calls = append(calls, "governor"); return nil },
		setSMT:             func(bool) error { calls = append(calls, "smt"); return nil },
	}

	command := newApplyProfileTestCommand(t)
	applyGPUClockOffset, applyGPUMemoryOffset = 100, 1000
	applyGPUPowerLimit, applyGPUFanSpeed = 350, 70
	applyCPUGovernor, applyCPUSMT = "performance", "on"
	for _, name := range []string{"gpu-clock-offset", "gpu-memory-offset", "gpu-power-limit", "gpu-fan-speed"} {
		if err := command.Flags().Set(name, "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := applySettings(command, operations); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "graphics,memory,power,fan,governor,smt" {
		t.Fatalf("apply calls = %s", got)
	}

	calls = nil
	applyGPUFanSpeed = 0
	if err := applyResetSettings(command, operations); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "reset clocks,power,reset fan,governor,smt" {
		t.Fatalf("reset calls = %s", got)
	}
	applyGPUFanSpeed = 70
	calls = nil
	if err := applyResetSettings(command, operations); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(calls, ","), "fan") {
		t.Fatalf("restore fan calls = %v", calls)
	}
}

func TestApplyProfileOperationErrorsRetainContext(t *testing.T) {
	command := newApplyProfileTestCommand(t)
	applyGPUClockOffset, applyGPUMemoryOffset = 1, 1
	applyGPUPowerLimit, applyGPUFanSpeed = 1, 1
	applyCPUGovernor, applyCPUSMT = "performance", "on"
	for _, name := range []string{"gpu-clock-offset", "gpu-memory-offset", "gpu-power-limit", "gpu-fan-speed"} {
		if err := command.Flags().Set(name, "1"); err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("fixture failure")
	newNoopOperations := func() applyProfileOperations {
		return applyProfileOperations{
			lockGraphicsClocks: func(int) error { return nil },
			lockMemoryClocks:   func(int) error { return nil },
			setPowerLimit:      func(int) error { return nil },
			setFanSpeed:        func(int) error { return nil },
			resetClocks:        func() error { return nil },
			resetFanSpeed:      func() error { return nil },
			setGovernor:        func(cpu.Governor) error { return nil },
			setSMT:             func(bool) error { return nil },
		}
	}
	tests := []struct {
		name string
		set  func(*applyProfileOperations)
		run  func(*cobra.Command, applyProfileOperations) error
		want string
	}{
		{"graphics", func(operations *applyProfileOperations) {
			operations.lockGraphicsClocks = func(int) error { return failure }
		}, applySettings, "GPU clock"},
		{"memory", func(operations *applyProfileOperations) {
			operations.lockMemoryClocks = func(int) error { return failure }
		}, applySettings, "GPU memory"},
		{"power", func(operations *applyProfileOperations) {
			operations.setPowerLimit = func(int) error { return failure }
		}, applySettings, "GPU power"},
		{"fan", func(operations *applyProfileOperations) {
			operations.setFanSpeed = func(int) error { return failure }
		}, applySettings, "GPU fan"},
		{"governor", func(operations *applyProfileOperations) {
			operations.setGovernor = func(cpu.Governor) error { return failure }
		}, applySettings, "CPU governor"},
		{"SMT", func(operations *applyProfileOperations) {
			operations.setSMT = func(bool) error { return failure }
		}, applySettings, "CPU SMT"},
		{"reset clocks", func(operations *applyProfileOperations) {
			operations.resetClocks = func() error { return failure }
		}, applyResetSettings, "reset GPU clocks"},
		{"reset power", func(operations *applyProfileOperations) {
			operations.setPowerLimit = func(int) error { return failure }
		}, applyResetSettings, "restore GPU power"},
		{"reset fan", func(operations *applyProfileOperations) {
			applyGPUFanSpeed = 0
			operations.resetFanSpeed = func() error { return failure }
		}, applyResetSettings, "reset GPU fan"},
		{"restore fan", func(operations *applyProfileOperations) {
			applyGPUFanSpeed = 1
			operations.setFanSpeed = func(int) error { return failure }
		}, applyResetSettings, "restore GPU fan"},
		{"restore governor", func(operations *applyProfileOperations) {
			operations.setGovernor = func(cpu.Governor) error { return failure }
		}, applyResetSettings, "restore CPU governor"},
		{"restore SMT", func(operations *applyProfileOperations) {
			operations.setSMT = func(bool) error { return failure }
		}, applyResetSettings, "restore CPU SMT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			applyGPUFanSpeed = 1
			operations := newNoopOperations()
			test.set(&operations)
			if err := test.run(command, operations); err == nil || !strings.Contains(err.Error(), test.want) || !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func newApplyProfileTestCommand(t *testing.T) *cobra.Command {
	t.Helper()
	command := &cobra.Command{}
	command.Flags().Int("gpu-clock-offset", 0, "")
	command.Flags().Int("gpu-memory-offset", 0, "")
	command.Flags().Int("gpu-power-limit", 0, "")
	command.Flags().Int("gpu-fan-speed", 0, "")
	return command
}
