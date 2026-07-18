package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestProfileYAMLContractCoversEveryLeaf(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	smt := false
	p := &Profile{
		Name:      "Cyberpunk contract",
		DLSS:      DLSSSettings{SRMode: DLSSModeQuality, SRPreset: DLSSPresetK, SROverride: true, RRMode: DLSSModeDLAA, RRPreset: DLSSPresetL, RROverride: true, FGEnabled: true, FGOverride: true, MultiFrame: 4, Indicator: true, FGIndicator: true},
		GPU:       GPUSettings{ShaderCache: true, ShaderCachePath: "/shader-cache", ThreadedOptimization: true, ClockOffset: 120, MemoryOffset: 500, PowerLimit: 300, PowerMizer: "prefer_maximum_performance", FanSpeed: 70},
		CPU:       CPUSettings{Governor: "performance", SMT: &smt, Affinity: "0-7"},
		Proton:    ProtonSettings{EnableWayland: true, EnableHDR: true, EnableNGXUpdater: true, VKD3DHeap: true},
		Overlay:   OverlaySettings{Enabled: true, Position: "top-right", ShowFPS: true, ShowFrametime: true, ShowCPU: true, ShowGPU: true, ShowVRAM: true, ToggleKey: "F12"},
		Overrides: allProfileFieldOverrides(),
	}
	if err := Save(1091500, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles", "1091500.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `name: Cyberpunk contract
dlss:
    sr_mode: quality
    sr_preset: K
    sr_override: true
    rr_mode: dlaa
    rr_preset: L
    rr_override: true
    fg_enabled: true
    fg_override: true
    multi_frame: 4
    indicator: true
    fg_indicator: true
gpu:
    shader_cache: true
    shader_cache_path: /shader-cache
    threaded_optimization: true
    clock_offset: 120
    memory_offset: 500
    power_limit: 300
    power_mizer: prefer_maximum_performance
    fan_speed: 70
cpu:
    governor: performance
    smt: false
    affinity: 0-7
proton:
    enable_wayland: true
    enable_hdr: true
    enable_ngx_updater: true
    vkd3d_heap: true
overlay:
    enabled: true
    position: top-right
    show_fps: true
    show_frametime: true
    show_cpu: true
    show_gpu: true
    show_vram: true
    toggle_key: F12
overrides:
    cpu.affinity: true
    cpu.governor: true
    cpu.smt: true
    dlss.fg_enabled: true
    dlss.fg_indicator: true
    dlss.fg_override: true
    dlss.indicator: true
    dlss.multi_frame: true
    dlss.rr_mode: true
    dlss.rr_override: true
    dlss.rr_preset: true
    dlss.sr_mode: true
    dlss.sr_override: true
    dlss.sr_preset: true
    gpu.clock_offset: true
    gpu.fan_speed: true
    gpu.memory_offset: true
    gpu.power_limit: true
    gpu.power_mizer: true
    gpu.shader_cache: true
    gpu.shader_cache_path: true
    gpu.threaded_optimization: true
    overlay.enabled: true
    overlay.position: true
    overlay.show_cpu: true
    overlay.show_fps: true
    overlay.show_frametime: true
    overlay.show_gpu: true
    overlay.show_vram: true
    overlay.toggle_key: true
    proton.enable_hdr: true
    proton.enable_ngx_updater: true
    proton.enable_wayland: true
    proton.vkd3d_heap: true
`
	if string(data) != want {
		t.Fatalf("persisted profile contract changed:\nwant:\n%s\ngot:\n%s", want, data)
	}
}

func TestProfileYAMLContractPreservesExplicitZeroValuesAndNilSMT(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := &Profile{Overrides: allProfileFieldOverrides()}
	if err := Save(1091500, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(1091500)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit false/zero/empty profile changed on round trip:\nwant: %#v\ngot:  %#v", want, got)
	}
	if got.CPU.SMT != nil {
		t.Fatalf("nil SMT became %#v", got.CPU.SMT)
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles", "1091500.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range AllFields() {
		if !strings.Contains(string(data), field+": true") {
			t.Errorf("explicit zero-value override %q missing from YAML:\n%s", field, data)
		}
	}
}

func TestProfileFieldRegistryMatchesYAMLLeaves(t *testing.T) {
	typeOfProfile := reflect.TypeOf(Profile{})
	var leaves []string
	for sectionIndex := 0; sectionIndex < typeOfProfile.NumField(); sectionIndex++ {
		section := typeOfProfile.Field(sectionIndex)
		if section.Type.Kind() != reflect.Struct {
			continue
		}
		sectionName := strings.Split(section.Tag.Get("yaml"), ",")[0]
		for leafIndex := 0; leafIndex < section.Type.NumField(); leafIndex++ {
			leafName := strings.Split(section.Type.Field(leafIndex).Tag.Get("yaml"), ",")[0]
			leaves = append(leaves, sectionName+"."+leafName)
		}
	}
	sort.Strings(leaves)
	fields := AllFields()
	sort.Strings(fields)
	if !reflect.DeepEqual(fields, leaves) {
		t.Fatalf("profile field registry does not exhaust YAML leaves:\nregistry: %v\nleaves:   %v", fields, leaves)
	}
}

func allProfileFieldOverrides() map[string]bool {
	overrides := make(map[string]bool, len(AllFields()))
	for _, field := range AllFields() {
		overrides[field] = true
	}
	return overrides
}
