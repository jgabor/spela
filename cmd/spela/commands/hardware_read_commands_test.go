package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/cpu"
)

func TestCPUReadCommandsProjectDeterministicFixtureState(t *testing.T) {
	operations := cpuReadOperations{}
	operations.getInfo = func() (map[string]string, error) {
		return map[string]string{"model": "Fixture CPU", "cores": "16", "governor": "performance", "smt": "true"}, nil
	}
	operations.getAvailableGovernors = func() ([]cpu.Governor, error) {
		return []cpu.Governor{cpu.GovernorPerformance, cpu.GovernorPowersave}, nil
	}
	operations.getMetrics = func() (*cpu.CPUMetrics, error) {
		return &cpu.CPUMetrics{AverageFrequency: 4200, Utilization: 25, RAMUsedMB: 8192, RAMTotalMB: 32768}, nil
	}
	operations.scxIsAvailable = func() bool { return true }
	operations.scxStatus = func() (bool, error) { return true, nil }
	operations.getCurrentGovernor = func() (cpu.Governor, error) { return cpu.GovernorPerformance, nil }
	operations.getSMTStatus = func() (bool, error) { return true, nil }

	for _, test := range []struct {
		run  func() error
		want []string
	}{
		{func() error { return runCPUInfoWithOperations(operations) }, []string{"Fixture CPU", "4200 MHz avg", "25%", "8192 / 32768 MB", "SCX:", "active"}},
		{func() error { return runCPUGovernorWithOperations(nil, operations) }, []string{"Current governor: performance", "Available: [performance powersave]"}},
		{func() error { return runCPUSMTWithOperations(nil, operations) }, []string{"SMT: true"}},
	} {
		output := captureStdout(t, func() {
			if err := test.run(); err != nil {
				t.Fatal(err)
			}
		})
		for _, want := range test.want {
			if !strings.Contains(output, want) {
				t.Errorf("CPU output missing %q:\n%s", want, output)
			}
		}
	}

	operations.getInfo = func() (map[string]string, error) { return nil, errors.New("fixture unavailable") }
	if err := runCPUInfoWithOperations(operations); err == nil || err.Error() != "fixture unavailable" {
		t.Fatalf("CPU fixture error = %v", err)
	}
}

func TestGPUReadCommandProjectsDeterministicFixtureAndError(t *testing.T) {
	getInfo := func() (map[string]string, error) {
		return map[string]string{
			"name": "Fixture GPU", "driver": "590.1", "memory": "32768 MB", "temperature": "55°C",
			"power_draw": "250 W", "power_limit": "450 W", "power_range": "300–500 W",
			"graphics_clock": "2800 MHz", "memory_clock": "14000 MHz", "fan_speed": "65%", "utilization": "98%",
		}, nil
	}
	output := captureStdout(t, func() {
		if err := runGPUInfoWith(getInfo); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"Fixture GPU", "250 W", "450 W limit", "300–500 W range", "2800 MHz", "14000 MHz", "65%", "98%"} {
		if !strings.Contains(output, want) {
			t.Errorf("GPU output missing %q:\n%s", want, output)
		}
	}
	getInfo = func() (map[string]string, error) { return nil, errors.New("fixture unavailable") }
	if err := runGPUInfoWith(getInfo); err == nil || !strings.Contains(err.Error(), "fixture unavailable") {
		t.Fatalf("GPU fixture error = %v", err)
	}
}
