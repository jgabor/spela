package main

import (
	"fmt"

	"github.com/jgabor/spela/internal/profile"
)

// fieldValue is one profile leaf and the value a setter gives it.
type fieldValue struct {
	Field string
	Value any
}

// contractValues are the values of TestProfileYAMLContractCoversEveryLeaf in
// internal/profile/yaml_contract_test.go: one non-zero value for every leaf.
var contractValues = []fieldValue{
	{profile.FieldDLSSSRMode, "quality"},
	{profile.FieldDLSSSRPreset, "K"},
	{profile.FieldDLSSSROverride, true},
	{profile.FieldDLSSRRMode, "dlaa"},
	{profile.FieldDLSSRRPreset, "L"},
	{profile.FieldDLSSRROverride, true},
	{profile.FieldDLSSFGEnabled, true},
	{profile.FieldDLSSFGOverride, true},
	{profile.FieldDLSSMultiFrame, 4},
	{profile.FieldDLSSIndicator, true},
	{profile.FieldDLSSFGIndicator, true},
	{profile.FieldGPUVRR, "automatic"},
	{profile.FieldGPUShaderCache, true},
	{profile.FieldGPUShaderCachePath, "/shader-cache"},
	{profile.FieldGPUThreadedOptimization, true},
	{profile.FieldGPUClockOffset, 120},
	{profile.FieldGPUMemoryOffset, 500},
	{profile.FieldGPUPowerLimit, 300},
	{profile.FieldGPUPowerMizer, "prefer_maximum_performance"},
	{profile.FieldGPUFanSpeed, 70},
	{profile.FieldCPUGovernor, "performance"},
	{profile.FieldCPUSMT, false},
	{profile.FieldCPUAffinity, "0-7"},
	{profile.FieldProtonEnableWayland, true},
	{profile.FieldProtonEnableHDR, true},
	{profile.FieldProtonEnableNGXUpdater, true},
	{profile.FieldProtonVKD3DHeap, true},
	{profile.FieldOverlayEnabled, true},
	{profile.FieldOverlayPosition, "top-right"},
	{profile.FieldOverlayShowFPS, true},
	{profile.FieldOverlayShowFrametime, true},
	{profile.FieldOverlayShowCPU, true},
	{profile.FieldOverlayShowGPU, true},
	{profile.FieldOverlayShowVRAM, true},
	{profile.FieldOverlayToggleKey, "F12"},
}

// allGamesEveryValue sets every field of All games, a mix of zero and
// non-zero values that differ from contractValues.
var allGamesEveryValue = []fieldValue{
	{profile.FieldDLSSSRMode, "performance"},
	{profile.FieldDLSSSRPreset, "auto"},
	{profile.FieldDLSSSROverride, true},
	{profile.FieldDLSSRRMode, "quality"},
	{profile.FieldDLSSRRPreset, "K"},
	{profile.FieldDLSSRROverride, false},
	{profile.FieldDLSSFGEnabled, false},
	{profile.FieldDLSSFGOverride, true},
	{profile.FieldDLSSMultiFrame, 2},
	{profile.FieldDLSSIndicator, false},
	{profile.FieldDLSSFGIndicator, false},
	{profile.FieldGPUVRR, "never"},
	{profile.FieldGPUShaderCache, false},
	{profile.FieldGPUShaderCachePath, ""},
	{profile.FieldGPUThreadedOptimization, false},
	{profile.FieldGPUClockOffset, 0},
	{profile.FieldGPUMemoryOffset, 0},
	{profile.FieldGPUPowerLimit, 250},
	{profile.FieldGPUPowerMizer, "adaptive"},
	{profile.FieldGPUFanSpeed, 0},
	{profile.FieldCPUGovernor, "schedutil"},
	{profile.FieldCPUSMT, true},
	{profile.FieldCPUAffinity, ""},
	{profile.FieldProtonEnableWayland, false},
	{profile.FieldProtonEnableHDR, true},
	{profile.FieldProtonEnableNGXUpdater, false},
	{profile.FieldProtonVKD3DHeap, false},
	{profile.FieldOverlayEnabled, false},
	{profile.FieldOverlayPosition, "bottom-left"},
	{profile.FieldOverlayShowFPS, true},
	{profile.FieldOverlayShowFrametime, false},
	{profile.FieldOverlayShowCPU, false},
	{profile.FieldOverlayShowGPU, false},
	{profile.FieldOverlayShowVRAM, false},
	{profile.FieldOverlayToggleKey, ""},
}

// saveGame writes a game profile the way the CLI setters do: a locked
// Mutate that fills the name and calls Set for each field.
func saveGame(appID uint64, name string, values []fieldValue) error {
	return profile.Mutate(appID, func(current, _ *profile.Profile) error {
		if current.Name == "" {
			current.Name = name
		}
		for _, value := range values {
			if err := current.Set(value.Field, value.Value); err != nil {
				return fmt.Errorf("set %s: %w", value.Field, err)
			}
		}
		return nil
	})
}

// saveAllGames writes All games the way the TUI's All games editor does:
// MutateDefault and Set.
func saveAllGames(values []fieldValue) error {
	return profile.MutateDefault(func(current *profile.Profile) error {
		for _, value := range values {
			if err := current.Set(value.Field, value.Value); err != nil {
				return fmt.Errorf("set %s: %w", value.Field, err)
			}
		}
		return nil
	})
}

// profileFile is the path of a profile file in the current case.
func profileFile(c *fixtureCase, name string) string {
	return c.path("config", "spela", "profiles", name)
}

func (w *fixtureWriter) profileOracle(c *fixtureCase, note string, appIDs ...uint64) error {
	oracle, err := profileOracle(c, note, appIDs)
	if err != nil {
		return err
	}
	return writeJSON(c.path("oracle.json"), oracle)
}

// noProfile is an app id no case writes a profile for, so each oracle also
// shows what a game without a profile resolves to.
const noProfile = 9100199

func (w *fixtureWriter) writeProfileCases() error {
	cases := []func() error{
		w.profileEveryField,
		w.profileExplicitZeros,
		w.profileOwnValues,
		w.profileLegacyGoTests,
		w.profileLegacyWithDefault,
		w.profileLegacyNoDefault,
		w.profileDefaultUnmarked,
		w.profileSRModelPreset,
		w.profileYAML11Bools,
		w.profileTwoSpace,
		w.profileYAMLShapes,
		w.profileEmptyOverrides,
		w.profileMarkers,
		w.profileDroppedFields,
		w.profileInvalidVRR,
		w.profileBroken,
		w.profileBrokenDefault,
		w.profileFileNames,
	}
	for _, run := range cases {
		if err := run(); err != nil {
			return err
		}
	}
	return nil
}

func (w *fixtureWriter) profileEveryField() error {
	c, err := w.enterCase("profiles/every-field")
	if err != nil {
		return err
	}
	if err := saveAllGames(allGamesEveryValue); err != nil {
		return err
	}
	if err := saveGame(9100101, "Glimmer Run", contractValues); err != nil {
		return err
	}
	w.note("profiles/every-field/", "Every one of the 35 fields set and marked, in All games (a mix of zero and non-zero values) and in one game (the values of Go's YAML contract test), 4-space indentation", "`profile.MutateDefault`, `profile.Mutate`, `(*Profile).Set`")
	return w.profileOracle(c, "every field set and marked in both scopes", 9100101, noProfile)
}

func (w *fixtureWriter) profileExplicitZeros() error {
	c, err := w.enterCase("profiles/explicit-zeros")
	if err != nil {
		return err
	}
	if err := saveAllGames(contractValues); err != nil {
		return err
	}
	allMarked := map[string]bool{}
	for _, field := range profile.AllFields() {
		allMarked[field] = true
	}
	// TestProfileYAMLContractPreservesExplicitZeroValuesAndNilSMT: every
	// leaf marked, every value zero, SMT nil.
	if err := profile.Save(9100102, &profile.Profile{Name: "Harbor Lights", Overrides: allMarked}); err != nil {
		return err
	}
	w.note("profiles/explicit-zeros/", "A game with every field marked and every value zero (`smt` nil, so marked with no value), against All games with non-zero values", "`profile.Save`, `profile.MutateDefault`")
	return w.profileOracle(c, "own zero values: each field is pinned to off, 0, empty or none", 9100102, noProfile)
}

func (w *fixtureWriter) profileOwnValues() error {
	c, err := w.enterCase("profiles/own-values")
	if err != nil {
		return err
	}
	if err := saveAllGames([]fieldValue{
		{profile.FieldProtonVKD3DHeap, true},
		{profile.FieldGPUPowerLimit, 300},
		{profile.FieldGPUThreadedOptimization, true},
		{profile.FieldCPUSMT, true},
		{profile.FieldDLSSSRPreset, "auto"},
	}); err != nil {
		return err
	}
	if err := saveGame(9100103, "Quiet Orchard", []fieldValue{
		{profile.FieldCPUSMT, false},
		{profile.FieldProtonVKD3DHeap, false},
		{profile.FieldGPUPowerLimit, 0},
		{profile.FieldGPUThreadedOptimization, false},
		{profile.FieldDLSSSRMode, "quality"},
		{profile.FieldDLSSSROverride, true},
	}); err != nil {
		return err
	}
	// Own values equal to All games, and an own value where All games
	// sets nothing but the value equals the system default.
	if err := saveGame(9100104, "Lantern Keep", []fieldValue{
		{profile.FieldProtonVKD3DHeap, true},
		{profile.FieldDLSSSRPreset, "auto"},
		{profile.FieldGPUFanSpeed, 0},
		{profile.FieldProtonEnableHDR, false},
	}); err != nil {
		return err
	}
	w.note("profiles/own-values/", "Own values with explicit zeros and `smt: false` that differ from All games, and own values equal to All games or to the system default (`this_game_same`)", "`profile.Mutate`, `profile.MutateDefault`, `(*Profile).Set`")
	return w.profileOracle(c, "this_game, this_game_same, all_games and system_default origins", 9100103, 9100104, noProfile)
}

func (w *fixtureWriter) profileLegacyGoTests() error {
	// TestMigration_LegacyProfileRoundTrip in internal/profile/inheritance_test.go.
	c, err := w.enterCase("profiles/legacy-go-test-hdr")
	if err != nil {
		return err
	}
	if err := profile.SaveDefault(&profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: true, EnableWayland: false},
		GPU:    profile.GPUSettings{PowerLimit: 300},
	}); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100210.yaml"), "name: Glass Harbor\nproton:\n  enable_hdr: true\n  enable_wayland: true\ngpu:\n  power_limit: 400\n"); err != nil {
		return err
	}
	w.note("profiles/legacy-go-test-hdr/", "Go test TestMigration_LegacyProfileRoundTrip: a 2-space legacy profile (no `overrides` key) against a default with values", "the Go test's YAML with a fictional app id and name in place of its real ones (decision 43); default by `profile.SaveDefault`")
	if err := w.profileOracle(c, "legacy migration: equal to default becomes inherited, different becomes an override", 9100210); err != nil {
		return err
	}

	// TestLoad_FileWithoutOverridesKeyStillMigrates in reset_persistence_test.go.
	c, err = w.enterCase("profiles/legacy-go-test-clock")
	if err != nil {
		return err
	}
	if err := profile.SaveDefault(&profile.Profile{GPU: profile.GPUSettings{ShaderCache: true, ThreadedOptimization: true, ClockOffset: 150, FanSpeed: 60}}); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100210.yaml"), "name: legacy\ngpu:\n  clock_offset: 150\n  fan_speed: 80\n"); err != nil {
		return err
	}
	w.note("profiles/legacy-go-test-clock/", "Go test TestLoad_FileWithoutOverridesKeyStillMigrates: legacy clock offset equal to the default, fan speed different", "the Go test's YAML with a fictional app id and name in place of its real ones (decision 43); default by `profile.SaveDefault`")
	if err := w.profileOracle(c, "legacy migration of GPU fields; the shader cache and threaded optimization the legacy file leaves out become own Off values", 9100210); err != nil {
		return err
	}

	// TestMigration_SkipsAlreadyMigrated in inheritance_test.go.
	c, err = w.enterCase("profiles/migrated-go-test")
	if err != nil {
		return err
	}
	if err := profile.SaveDefault(&profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100210.yaml"), "name: test\nproton:\n  vkd3d_heap: false\noverrides:\n  proton.vkd3d_heap: true\n"); err != nil {
		return err
	}
	w.note("profiles/migrated-go-test/", "Go test TestMigration_SkipsAlreadyMigrated: a 2-space profile with an `overrides` map pinning `vkd3d_heap: false` against a default of true", "the Go test's YAML with a fictional app id and name in place of its real ones (decision 43); default by `profile.SaveDefault`")
	return w.profileOracle(c, "an overrides map stops legacy migration; the explicit false survives", 9100210)
}

// legacyGameYAML is a game profile as Go wrote it before the overrides map
// existed: 4-space, omitempty, no overrides key.
const legacyGameYAML = `name: Ember Coast
dlss:
    sr_mode: balanced
    sr_preset: default
    sr_override: true
    multi_frame: 2
gpu:
    vrr: unset
    clock_offset: 150
    memory_offset: 9501
    power_limit: 250
    power_mizer: max
cpu:
    governor: performance
    smt: false
    affinity: 0-3
proton:
    enable_hdr: true
overlay:
    enabled: true
    position: top-left
`

func (w *fixtureWriter) profileLegacyWithDefault() error {
	c, err := w.enterCase("profiles/legacy-with-default")
	if err != nil {
		return err
	}
	// The All games file is legacy too (no overrides key); what it sets
	// is what is non-zero.
	defaultYAML := `dlss:
    sr_mode: balanced
    sr_preset: auto
proton:
    enable_hdr: true
    enable_wayland: true
gpu:
    power_limit: 250
cpu:
    governor: powersave
    smt: true
`
	if err := writeText(profileFile(c, "default.yaml"), defaultYAML); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100105.yaml"), legacyGameYAML); err != nil {
		return err
	}
	w.note("profiles/legacy-with-default/", "A 4-space legacy game profile (no `overrides` key) with `vrr: unset`, `smt: false`, non-zero clock and memory offsets and dropped fields, against a legacy `default.yaml` with values", "literal YAML in the shape Go wrote before the overrides map")
	return w.profileOracle(c, "legacy migration against a default: equal values become inherited, a key the game leaves out but the default sets becomes an own zero, vrr is explicit whenever present", 9100105, noProfile)
}

func (w *fixtureWriter) profileLegacyNoDefault() error {
	c, err := w.enterCase("profiles/legacy-no-default")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100105.yaml"), legacyGameYAML); err != nil {
		return err
	}
	w.note("profiles/legacy-no-default/", "The same legacy game profile with no `default.yaml`: every non-zero value becomes an own value", "literal YAML in the shape Go wrote before the overrides map")
	return w.profileOracle(c, "legacy migration with no default profile", 9100105, noProfile)
}

func (w *fixtureWriter) profileDefaultUnmarked() error {
	c, err := w.enterCase("profiles/default-unmarked")
	if err != nil {
		return err
	}
	smt := false
	// Values with no markers: Go writes overrides: {}.
	if err := profile.SaveDefault(&profile.Profile{
		Proton: profile.ProtonSettings{EnableWayland: true},
		DLSS:   profile.DLSSSettings{SRPreset: "auto", MultiFrame: 3},
		GPU:    profile.GPUSettings{FanSpeed: 0, VRR: "always"},
		CPU:    profile.CPUSettings{Governor: "performance", SMT: &smt},
	}); err != nil {
		return err
	}
	if err := saveGame(9100106, "Paper Comet", []fieldValue{{profile.FieldDLSSSRPreset, "J"}}); err != nil {
		return err
	}
	w.note("profiles/default-unmarked/", "All games with set values and no markers (`overrides: {}`), which Go counts as set when non-zero; `smt: false` counts as set", "`profile.SaveDefault`, `profile.Mutate`")
	if err := w.profileOracle(c, "unmarked All games values count as set when non-zero", 9100106, noProfile); err != nil {
		return err
	}

	c, err = w.enterCase("profiles/default-no-overrides-key")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "default.yaml"), "proton:\n    enable_hdr: true\ncpu:\n    smt: false\ngpu:\n    vrr: never\n"); err != nil {
		return err
	}
	if err := saveGame(9100106, "Paper Comet", []fieldValue{{profile.FieldProtonEnableHDR, false}}); err != nil {
		return err
	}
	w.note("profiles/default-no-overrides-key/", "A `default.yaml` with no `overrides` key at all (as the Go e2e fixture writes it)", "literal YAML; game by `profile.Mutate`")
	return w.profileOracle(c, "All games without an overrides key: Go never migrates it, non-zero values count as set", 9100106, noProfile)
}

func (w *fixtureWriter) profileSRModelPreset() error {
	c, err := w.enterCase("profiles/sr-model-preset")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "default.yaml"), "dlss:\n    sr_model_preset: m\noverrides:\n    dlss.sr_model_preset: true\n"); err != nil {
		return err
	}
	// TestUnmarshalProfileYAML_LegacyField, plus the marker.
	if err := writeText(profileFile(c, "9100108.yaml"), "name: Signal Drift\ndlss:\n  sr_preset: A\n  sr_model_preset: l\noverrides:\n  dlss.sr_model_preset: true\n"); err != nil {
		return err
	}
	// Legacy file (no overrides key) with only the old field.
	if err := writeText(profileFile(c, "9100109.yaml"), "name: Tidal Engine\ndlss:\n    sr_model_preset: auto\n    sr_override: true\n"); err != nil {
		return err
	}
	// Lowercase preset, normalized on load; an unknown model preset.
	if err := writeText(profileFile(c, "9100110.yaml"), "name: Copper Valley\ndlss:\n    sr_preset: k\n    sr_model_preset: transformer\noverrides:\n    dlss.sr_preset: true\n"); err != nil {
		return err
	}
	w.note("profiles/sr-model-preset/", "The retired `dlss.sr_model_preset` key and marker in All games and games, a legacy file with only that key, and a lowercase `sr_preset: k`", "literal YAML (Go no longer writes `sr_model_preset`)")
	return w.profileOracle(c, "Go folds sr_model_preset into sr_preset on load and moves its marker", 9100108, 9100109, 9100110, noProfile)
}

func (w *fixtureWriter) profileYAML11Bools() error {
	c, err := w.enterCase("profiles/yaml11-bools")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100111.yaml"), "name: Moss Signal\nproton:\n    enable_hdr: on\n    enable_wayland: Yes\n    vkd3d_heap: off\ncpu:\n    smt: yes\noverrides:\n    proton.enable_hdr: yes\n    proton.enable_wayland: true\n    proton.vkd3d_heap: true\n    cpu.smt: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100112.yaml"), "name: Moss Signal Y\ndlss:\n    sr_override: y\noverrides:\n    dlss.sr_override: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100113.yaml"), "name: Moss Signal Upper\ndlss:\n    fg_enabled: TRUE\n    fg_override: NO\noverrides:\n    dlss.fg_enabled: true\n    dlss.fg_override: true\n"); err != nil {
		return err
	}
	w.note("profiles/yaml11-bools/", "YAML 1.1 booleans: `smt: yes`, `on`, `off`, `Yes`, `y`, `TRUE`, `NO`, also as a marker value", "literal YAML, read by `profile.Load`")
	return w.profileOracle(c, "which YAML 1.1 boolean spellings Go's yaml.v3 accepts", 9100111, 9100112, 9100113)
}

func (w *fixtureWriter) profileTwoSpace() error {
	c, err := w.enterCase("profiles/two-space")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "default.yaml"), "proton:\n  enable_wayland: true\ngpu:\n  power_limit: 280\noverrides:\n  proton.enable_wayland: true\n  gpu.power_limit: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100114.yaml"), "name: Velvet Static\ndlss:\n  sr_mode: dlaa\n  sr_override: true\ngpu:\n  power_limit: 0\n  vrr: automatic\ncpu:\n  smt: false\noverrides:\n  dlss.sr_mode: true\n  dlss.sr_override: true\n  gpu.power_limit: true\n  gpu.vrr: true\n  cpu.smt: true\n"); err != nil {
		return err
	}
	w.note("profiles/two-space/", "2-space indented `default.yaml` and game profile with an overrides map, as Go's tests write them by hand", "literal YAML, read by `profile.Load`")
	return w.profileOracle(c, "2-space files load like 4-space ones", 9100114, noProfile)
}

func (w *fixtureWriter) profileYAMLShapes() error {
	c, err := w.enterCase("profiles/yaml-shapes")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100131.yaml"), "name: Copper Lantern\ngpu:\n    power_limit: 250.0\n    fan_speed: 40.7\ndlss:\n    multi_frame: 2.0\noverrides:\n    gpu.power_limit: true\n    gpu.fan_speed: true\n    dlss.multi_frame: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100132.yaml"), "name: Second Draft\nproton:\n    enable_hdr: true\noverrides:\n    proton.enable_hdr: true\n---\nname: Ignored\nproton:\n    enable_wayland: true\noverrides:\n    proton.enable_wayland: true\n"); err != nil {
		return err
	}
	w.note("profiles/yaml-shapes/", "Shapes yaml.v3 loads that a strict reader refuses: floats in int fields (`power_limit: 250.0`, `fan_speed: 40.7`, cut toward zero) and a second `---` document, which yaml.v3 ignores", "literal YAML, read by `profile.Load`")
	return w.profileOracle(c, "yaml.v3 reads floats into ints and only the first document", 9100131, 9100132)
}

func (w *fixtureWriter) profileEmptyOverrides() error {
	c, err := w.enterCase("profiles/overrides-empty-map")
	if err != nil {
		return err
	}
	if err := profile.SaveDefault(&profile.Profile{GPU: profile.GPUSettings{PowerLimit: 220}}); err != nil {
		return err
	}
	// Leaves with no marker: Go writes them, and they have no effect.
	if err := profile.Save(9100115, &profile.Profile{
		Name:   "Hollow Bell",
		Proton: profile.ProtonSettings{EnableHDR: true},
		GPU:    profile.GPUSettings{PowerLimit: 200},
	}); err != nil {
		return err
	}
	// TestSave_AlwaysWritesOverridesKey.
	if err := profile.Save(9100116, &profile.Profile{Name: "empty"}); err != nil {
		return err
	}
	w.note("profiles/overrides-empty-map/", "`overrides: {}` with stored leaves that have no marker (no effect in Go), and an empty profile with only a name", "`profile.Save`, `profile.SaveDefault`")
	return w.profileOracle(c, "unmarked leaves in a game profile have no effect", 9100115, 9100116, noProfile)
}

func (w *fixtureWriter) profileMarkers() error {
	c, err := w.enterCase("profiles/markers")
	if err != nil {
		return err
	}
	if err := profile.SaveDefault(&profile.Profile{
		Proton:    profile.ProtonSettings{EnableHDR: true},
		Overrides: map[string]bool{profile.FieldProtonEnableHDR: true, "made.up": true},
	}); err != nil {
		return err
	}
	if err := profile.Save(9100117, &profile.Profile{
		Name:   "Rook Harbor",
		Proton: profile.ProtonSettings{EnableHDR: true, EnableWayland: true},
		GPU:    profile.GPUSettings{PowerLimit: 180},
		Overrides: map[string]bool{
			profile.FieldProtonEnableHDR:     false,
			profile.FieldProtonEnableWayland: true,
			profile.FieldGPUPowerLimit:       true,
			"dlss.sr_model_preset":           false,
			"overlay.bogus":                  true,
			"made.up":                        true,
		},
	}); err != nil {
		return err
	}
	w.note("profiles/markers/", "`false` markers, unknown marker keys (`made.up`, `overlay.bogus`) and a `false` `dlss.sr_model_preset` marker, in All games and a game", "`profile.Save`, `profile.SaveDefault` with a hand-built overrides map")
	return w.profileOracle(c, "false markers follow; unknown markers are kept in the map and ignored", 9100117, noProfile)
}

func (w *fixtureWriter) profileDroppedFields() error {
	c, err := w.enterCase("profiles/dropped-fields")
	if err != nil {
		return err
	}
	if err := saveAllGames([]fieldValue{
		{profile.FieldOverlayEnabled, true},
		{profile.FieldOverlayPosition, "bottom-right"},
		{profile.FieldGPUPowerMizer, "adaptive"},
		{profile.FieldGPUClockOffset, 100},
		{profile.FieldGPUMemoryOffset, 8000},
	}); err != nil {
		return err
	}
	if err := saveGame(9100118, "Kestrel Lines", []fieldValue{
		{profile.FieldOverlayEnabled, true},
		{profile.FieldOverlayPosition, "top-left"},
		{profile.FieldOverlayShowFPS, true},
		{profile.FieldOverlayShowFrametime, true},
		{profile.FieldOverlayShowCPU, true},
		{profile.FieldOverlayShowGPU, false},
		{profile.FieldOverlayShowVRAM, true},
		{profile.FieldOverlayToggleKey, "Shift+F12"},
		{profile.FieldGPUPowerMizer, "prefer_max_performance"},
		{profile.FieldCPUAffinity, "0-7,16-23"},
		{profile.FieldGPUClockOffset, 150},
		{profile.FieldGPUMemoryOffset, 9501},
		{profile.FieldGPUPowerLimit, 320},
	}); err != nil {
		return err
	}
	w.note("profiles/dropped-fields/", "Every field decision 10 drops (8 overlay fields, `gpu.power_mizer`, `cpu.affinity`) set in a game and some in All games, plus non-zero `clock_offset` and `memory_offset` (decision 31)", "`profile.Mutate`, `profile.MutateDefault`, `(*Profile).Set`")
	return w.profileOracle(c, "dropped fields and Go's clock and memory offsets", 9100118, noProfile)
}

func (w *fixtureWriter) profileInvalidVRR() error {
	c, err := w.enterCase("profiles/invalid-vrr")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100119.yaml"), "name: Drift Atlas\ngpu:\n    vrr: sometimes\noverrides:\n    gpu.vrr: true\n"); err != nil {
		return err
	}
	if err := saveGame(9100120, "Drift Atlas Two", []fieldValue{{profile.FieldGPUVRR, "never"}}); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "default.yaml"), "gpu:\n    vrr: Always\n"); err != nil {
		return err
	}
	w.note("profiles/invalid-vrr/", "An invalid `gpu.vrr` in a game and a wrongly cased one in `default.yaml`: Go fails to load both", "literal YAML; valid sibling by `profile.Mutate`")
	return w.profileOracle(c, "gpu.vrr is the only value Go validates on load", 9100119, 9100120)
}

func (w *fixtureWriter) profileBroken() error {
	c, err := w.enterCase("profiles/broken-yaml")
	if err != nil {
		return err
	}
	// From TestProfileStorageSupportedFilesystemAndListingBoundaries.
	if err := writeText(profileFile(c, "9100121.yaml"), "not: [yaml"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100122.yaml"), "name: Wrong Type\ngpu:\n    power_limit: high\noverrides:\n    gpu.power_limit: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100123.yaml"), "name: Duplicate Keys\nproton:\n    enable_hdr: true\n    enable_hdr: false\noverrides: {}\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100124.yaml"), "name: Tabbed\nproton:\n\tenable_hdr: true\n"); err != nil {
		return err
	}
	if err := saveGame(9100125, "Still Fine", []fieldValue{{profile.FieldProtonEnableHDR, true}}); err != nil {
		return err
	}
	w.note("profiles/broken-yaml/", "Game profiles that do not load: unparseable YAML, a wrong type, duplicate keys, tab indentation; `profile.List` skips them", "literal YAML; valid sibling by `profile.Mutate`")
	return w.profileOracle(c, "load errors are per file; List skips broken files silently", 9100121, 9100122, 9100123, 9100124, 9100125)
}

func (w *fixtureWriter) profileBrokenDefault() error {
	c, err := w.enterCase("profiles/broken-default")
	if err != nil {
		return err
	}
	if err := writeText(profileFile(c, "default.yaml"), "proton: [broken\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100126.yaml"), "name: Needs Default\nproton:\n    enable_hdr: true\n"); err != nil {
		return err
	}
	if err := writeText(profileFile(c, "9100127.yaml"), "name: Own Map\nproton:\n    enable_hdr: true\noverrides:\n    proton.enable_hdr: true\n"); err != nil {
		return err
	}
	w.note("profiles/broken-default/", "A `default.yaml` that does not parse: the legacy game profile fails with it, the one with an overrides map still loads", "literal YAML")
	return w.profileOracle(c, "a broken default fails legacy game profiles only; launch resolution needs the default and fails for all", 9100126, 9100127, noProfile)
}

func (w *fixtureWriter) profileFileNames() error {
	c, err := w.enterCase("profiles/file-names")
	if err != nil {
		return err
	}
	if err := saveGame(9100128, "Plain Yaml", []fieldValue{{profile.FieldProtonEnableHDR, true}}); err != nil {
		return err
	}
	for name, content := range map[string]string{
		"9100129.yml":   "name: Yml Extension\nproton:\n    enable_hdr: true\noverrides:\n    proton.enable_hdr: true\n",
		"09100130.yaml": "name: Leading Zero\nproton:\n    enable_hdr: true\noverrides:\n    proton.enable_hdr: true\n",
		"notes.yaml":    "name: Not A Game\n",
		"default.yml":   "proton:\n    enable_hdr: true\n",
		"README.txt":    "not a profile\n",
	} {
		if err := writeText(profileFile(c, name), content); err != nil {
			return err
		}
	}
	w.note("profiles/file-names/", "File names Go's List and Load treat differently: `.yml`, a leading zero stem, a non-numeric stem, `default.yml`, a text file", "literal YAML; valid profile by `profile.Mutate`")
	return w.profileOracle(c, "List parses .yml and leading-zero stems, but Load only opens <appid>.yaml, so they never take effect", 9100128, 9100129, 9100130)
}
