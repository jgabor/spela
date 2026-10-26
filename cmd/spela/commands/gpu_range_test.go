package commands

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
	"github.com/spf13/cobra"
)

func TestGPUSetRefusesValuesOutsideTheFieldRange(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	for _, test := range []struct {
		flag  string
		value *int
		want  string
	}{
		{"fan-speed", &gpuSetFanSpeed, "must be 0 to 100 %, got 150"},
		{"power-limit", &gpuSetPowerLimit, "must be at least 0 W, got -10"},
	} {
		command := &cobra.Command{}
		for _, name := range []string{"clock-offset", "memory-offset", "power-limit", "fan-speed"} {
			command.Flags().Int(name, 0, "")
		}
		gpuSetPowerMizer, gpuSetShaderCache, gpuSetCachePath, gpuSetThreadedOpt = "", "", "", ""
		*test.value = 150
		if test.flag == "power-limit" {
			*test.value = -10
		}
		if err := command.Flags().Set(test.flag, "1"); err != nil {
			t.Fatal(err)
		}
		err := runGPUSet(command, []string{"Cyberpunk 2077"})
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("--%s error = %v, want %q", test.flag, err, test.want)
		}
		*test.value = 0
	}
	stored, err := profile.Load(1091500)
	if err != nil || (stored != nil && (stored.GPU.FanSpeed != 0 || stored.GPU.PowerLimit != 0)) {
		t.Fatalf("a refused value must not be saved: %+v, %v", stored, err)
	}
}

func TestDryRunAgreesThatAZeroFanSpeedAppliesNothing(t *testing.T) {
	withTempXDG(t)
	seedGameWithDLL(t)
	defaults := &profile.Profile{}
	defaults.GPU.FanSpeed = 70
	defaults.MarkOverride(profile.FieldGPUFanSpeed)
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatal(err)
	}
	gameProfile := &profile.Profile{}
	if err := gameProfile.Set(profile.FieldGPUFanSpeed, 0); err != nil {
		t.Fatal(err)
	}
	if err := profile.Save(1091500, gameProfile); err != nil {
		t.Fatal(err)
	}
	launchGameID, launchDryRun = 0, true
	t.Cleanup(func() { launchGameID, launchDryRun = 0, false })

	output := captureStdout(t, func() {
		if err := runLaunch(LaunchCmd, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{
		"gpu.fan_speed=0 source=override restore=not_applicable (nothing applied: 0 leaves it unchanged)",
		"no privileged hardware mutation planned",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("dry run is missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "restore=restorable_mutation") && strings.Contains(output, "gpu.fan_speed=0 source=override restore=restorable_mutation") {
		t.Errorf("dry run still claims a restorable fan mutation:\n%s", output)
	}
}
