package tui

import (
	"fmt"
	"strconv"

	"github.com/jgabor/spela/internal/profile"
)

// Display helper functions for profile field values.

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intStr(i int) string {
	return strconv.Itoa(i)
}

func srPresetValue(p profile.DLSSPreset) string {
	if p == "" {
		return "default"
	}
	return string(p)
}

func powerMizerValue(p string) string {
	if p == "" {
		return "auto"
	}
	return p
}

func displayValue(v string) string {
	if v == "" || v == "default" || v == "auto" {
		return "(default)"
	}
	return v
}

func displayBool(b bool) string {
	if !b {
		return "(default)"
	}
	return "true"
}

func displayBoolPtr(b *bool) string {
	if b == nil {
		return "(default)"
	}
	if *b {
		return "true"
	}
	return "false"
}

func displayFrameGeneration(enabled bool, override bool) string {
	if !override {
		return "(default)"
	}
	if enabled {
		return "true"
	}
	return "false"
}

func displayInt(i int) string {
	if i == 0 {
		return "(default)"
	}
	return strconv.Itoa(i)
}

// newProfileWidget constructs a ProfileWidgetModel with all field group definitions.
func newProfileWidget(saveTarget ProfileSaveTarget, name string, p *profile.Profile, styles *Styles) ProfileWidgetModel {
	if p == nil {
		p = &profile.Profile{Name: name}
	}

	groups := []WidgetGroup{
		{
			title: "DLSS super resolution",
			fields: []WidgetField{
				{
					label:       "Quality mode",
					key:         "sr_mode",
					value:       displayValue(string(p.DLSS.SRMode)),
					options:     []string{"(default)", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
					description: "Super resolution quality mode",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.SRMode = ""
						} else {
							p.DLSS.SRMode = profile.DLSSMode(v)
						}
					},
				},
				{
					label:       "DLSS preset",
					key:         "sr_preset",
					value:       displayValue(srPresetValue(p.DLSS.SRPreset)),
					options:     []string{"(default)", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
					description: "Neural network preset (A-F: CNN, J-M: Transformer)",
					usesModal:   true,
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.SRPreset = ""
						} else {
							p.DLSS.SRPreset = profile.DLSSPreset(v)
						}
					},
				},
				{
					label:       "Model preset",
					key:         "sr_model_preset",
					value:       displayValue(string(p.DLSS.SRModelPreset)),
					options:     []string{"(default)", "auto", "k", "l", "m"},
					description: "Force specific transformer model version",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.SRModelPreset = ""
						} else {
							p.DLSS.SRModelPreset = profile.DLSSModelPreset(v)
						}
					},
				},
				{
					label:       "Override",
					key:         "sr_override",
					value:       displayBool(p.DLSS.SROverride),
					options:     []string{"(default)", "true", "false"},
					description: "Force DLSS even if unsupported",
					apply: func(p *profile.Profile, v string, d bool) {
						p.DLSS.SROverride = v == "true"
					},
				},
				{
					label:       "SR indicator",
					key:         "indicator",
					value:       displayBool(p.DLSS.Indicator),
					options:     []string{"(default)", "true", "false"},
					description: "Show on-screen DLSS indicator",
					apply: func(p *profile.Profile, v string, d bool) {
						p.DLSS.Indicator = v == "true"
					},
				},
			},
		},
		{
			title: "DLSS ray reconstruction",
			fields: []WidgetField{
				{
					label:       "RR mode",
					key:         "rr_mode",
					value:       displayValue(string(p.DLSS.RRMode)),
					options:     []string{"(default)", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"},
					description: "Ray reconstruction quality mode",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.RRMode = ""
						} else {
							p.DLSS.RRMode = profile.DLSSMode(v)
						}
					},
				},
				{
					label:       "RR preset",
					key:         "rr_preset",
					value:       displayValue(srPresetValue(p.DLSS.RRPreset)),
					options:     []string{"(default)", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"},
					description: "Ray reconstruction neural network preset",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.RRPreset = ""
						} else {
							p.DLSS.RRPreset = profile.DLSSPreset(v)
						}
					},
				},
				{
					label:       "RR override",
					key:         "rr_override",
					value:       displayBool(p.DLSS.RROverride),
					options:     []string{"(default)", "true", "false"},
					description: "Force ray reconstruction even if unsupported",
					apply: func(p *profile.Profile, v string, d bool) {
						p.DLSS.RROverride = v == "true"
					},
				},
			},
		},
		{
			title: "DLSS frame generation",
			fields: []WidgetField{
				{
					label:       "Frame gen",
					key:         "fg_enabled",
					value:       displayFrameGeneration(p.DLSS.FGEnabled, p.DLSS.FGOverride),
					options:     []string{"(default)", "true", "false"},
					description: "Enable AI frame generation",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.FGEnabled = false
							p.DLSS.FGOverride = false
						} else {
							p.DLSS.FGEnabled = v == "true"
							p.DLSS.FGOverride = true
						}
					},
				},
				{
					label:       "Multi-frame",
					key:         "multi_frame",
					value:       displayInt(p.DLSS.MultiFrame),
					options:     []string{"(default)", "1", "2", "3", "4"},
					description: "Extra frames to generate (0=off)",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.DLSS.MultiFrame = 0
						} else {
							var n int
							_, _ = fmt.Sscanf(v, "%d", &n)
							p.DLSS.MultiFrame = n
						}
					},
				},
				{
					label:       "FG indicator",
					key:         "fg_indicator",
					value:       displayBool(p.DLSS.FGIndicator),
					options:     []string{"(default)", "true", "false"},
					description: "Show on-screen frame generation indicator",
					apply: func(p *profile.Profile, v string, d bool) {
						p.DLSS.FGIndicator = v == "true"
					},
				},
			},
		},
		{
			title: "GPU settings",
			fields: []WidgetField{
				{
					label:       "Shader cache",
					key:         "shader_cache",
					value:       displayBool(p.GPU.ShaderCache),
					options:     []string{"(default)", "true", "false"},
					description: "Enable GPU shader caching",
					apply: func(p *profile.Profile, v string, d bool) {
						p.GPU.ShaderCache = v == "true"
					},
				},
				{
					label:       "Shader cache path",
					key:         "shader_cache_path",
					value:       displayValue(p.GPU.ShaderCachePath),
					options:     nil,
					description: "Custom path for shader cache storage",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.ShaderCachePath = ""
						} else {
							p.GPU.ShaderCachePath = v
						}
					},
				},
				{
					label:       "Threaded opt",
					key:         "threaded_opt",
					value:       displayBool(p.GPU.ThreadedOptimization),
					options:     []string{"(default)", "true", "false"},
					description: "Enable threaded optimization",
					apply: func(p *profile.Profile, v string, d bool) {
						p.GPU.ThreadedOptimization = v == "true"
					},
				},
				{
					label:       "Power mode",
					key:         "power_mizer",
					value:       displayValue(powerMizerValue(p.GPU.PowerMizer)),
					options:     []string{"(default)", "adaptive", "max"},
					description: "GPU power mode",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.PowerMizer = ""
						} else {
							p.GPU.PowerMizer = v
						}
					},
				},
				{
					label:       "Clock offset",
					key:         "clock_offset",
					value:       displayInt(p.GPU.ClockOffset),
					options:     []string{"(default)", "-200", "-100", "-50", "50", "100", "150", "200", "250", "300"},
					description: "GPU core clock offset in MHz",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.ClockOffset = 0
						} else {
							var n int
							_, _ = fmt.Sscanf(v, "%d", &n)
							p.GPU.ClockOffset = n
						}
					},
				},
				{
					label:       "Memory offset",
					key:         "memory_offset",
					value:       displayInt(p.GPU.MemoryOffset),
					options:     []string{"(default)", "-500", "-200", "200", "500", "750", "1000"},
					description: "GPU memory clock offset in MHz",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.MemoryOffset = 0
						} else {
							var n int
							_, _ = fmt.Sscanf(v, "%d", &n)
							p.GPU.MemoryOffset = n
						}
					},
				},
				{
					label:       "Power limit",
					key:         "power_limit",
					value:       displayInt(p.GPU.PowerLimit),
					options:     []string{"(default)", "150", "200", "250", "300", "350", "400", "450", "500", "600"},
					description: "GPU power limit in watts",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.PowerLimit = 0
						} else {
							var n int
							_, _ = fmt.Sscanf(v, "%d", &n)
							p.GPU.PowerLimit = n
						}
					},
				},
				{
					label:       "Fan speed",
					key:         "fan_speed",
					value:       displayInt(p.GPU.FanSpeed),
					options:     []string{"(default)", "30", "40", "50", "60", "70", "80", "90", "100"},
					description: "GPU fan speed percentage (0 = auto)",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.GPU.FanSpeed = 0
						} else {
							var n int
							_, _ = fmt.Sscanf(v, "%d", &n)
							p.GPU.FanSpeed = n
						}
					},
				},
			},
		},
		{
			title: "CPU settings",
			fields: []WidgetField{
				{
					label:       "Governor",
					key:         "cpu_governor",
					value:       displayValue(p.CPU.Governor),
					options:     []string{"(default)", "performance", "powersave", "schedutil", "ondemand"},
					description: "CPU frequency scaling governor",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.CPU.Governor = ""
						} else {
							p.CPU.Governor = v
						}
					},
				},
				{
					label:       "SMT",
					key:         "cpu_smt",
					value:       displayBoolPtr(p.CPU.SMT),
					options:     []string{"(default)", "true", "false"},
					description: "Enable simultaneous multi-threading (hyperthreading)",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.CPU.SMT = nil
						} else {
							b := v == "true"
							p.CPU.SMT = &b
						}
					},
				},
				{
					label:       "Affinity",
					key:         "cpu_affinity",
					value:       displayValue(p.CPU.Affinity),
					options:     nil,
					description: "CPU core affinity mask (hex or decimal)",
					disabled:    true,
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.CPU.Affinity = ""
						} else {
							p.CPU.Affinity = v
						}
					},
				},
			},
		},
		{
			title: "Proton settings",
			fields: []WidgetField{
				{
					label:       "HDR",
					key:         "hdr",
					value:       displayBool(p.Proton.EnableHDR),
					options:     []string{"(default)", "true", "false"},
					description: "Enable high dynamic range",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Proton.EnableHDR = v == "true"
					},
				},
				{
					label:       "Wayland",
					key:         "wayland",
					value:       displayBool(p.Proton.EnableWayland),
					options:     []string{"(default)", "true", "false"},
					description: "Use native Wayland",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Proton.EnableWayland = v == "true"
					},
				},
				{
					label:       "NGX updater",
					key:         "ngx_updater",
					value:       displayBool(p.Proton.EnableNGXUpdater),
					options:     []string{"(default)", "true", "false"},
					description: "Auto-update DLSS DLLs",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Proton.EnableNGXUpdater = v == "true"
					},
				},
			},
		},
		{
			title: "Overlay settings",
			fields: []WidgetField{
				{
					label:       "Enabled",
					key:         "overlay_enabled",
					value:       displayBool(p.Overlay.Enabled),
					options:     []string{"(default)", "true", "false"},
					description: "Show performance overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.Enabled = v == "true"
					},
				},
				{
					label:       "Position",
					key:         "overlay_position",
					value:       displayValue(p.Overlay.Position),
					options:     []string{"(default)", "top-left", "top-right", "bottom-left", "bottom-right"},
					description: "Overlay screen position",
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.Overlay.Position = ""
						} else {
							p.Overlay.Position = v
						}
					},
				},
				{
					label:       "Show FPS",
					key:         "overlay_fps",
					value:       displayBool(p.Overlay.ShowFPS),
					options:     []string{"(default)", "true", "false"},
					description: "Show frames per second in overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.ShowFPS = v == "true"
					},
				},
				{
					label:       "Show frametime",
					key:         "overlay_frametime",
					value:       displayBool(p.Overlay.ShowFrametime),
					options:     []string{"(default)", "true", "false"},
					description: "Show frame time in overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.ShowFrametime = v == "true"
					},
				},
				{
					label:       "Show CPU",
					key:         "overlay_cpu",
					value:       displayBool(p.Overlay.ShowCPU),
					options:     []string{"(default)", "true", "false"},
					description: "Show CPU usage in overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.ShowCPU = v == "true"
					},
				},
				{
					label:       "Show GPU",
					key:         "overlay_gpu",
					value:       displayBool(p.Overlay.ShowGPU),
					options:     []string{"(default)", "true", "false"},
					description: "Show GPU usage in overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.ShowGPU = v == "true"
					},
				},
				{
					label:       "Show VRAM",
					key:         "overlay_vram",
					value:       displayBool(p.Overlay.ShowVRAM),
					options:     []string{"(default)", "true", "false"},
					description: "Show VRAM usage in overlay",
					apply: func(p *profile.Profile, v string, d bool) {
						p.Overlay.ShowVRAM = v == "true"
					},
				},
				{
					label:       "Toggle key",
					key:         "overlay_toggle_key",
					value:       displayValue(p.Overlay.ToggleKey),
					options:     nil,
					description: "Key to toggle overlay visibility",
					disabled:    true,
					apply: func(p *profile.Profile, v string, d bool) {
						if d {
							p.Overlay.ToggleKey = ""
						} else {
							p.Overlay.ToggleKey = v
						}
					},
				},
			},
		},
	}

	return ProfileWidgetModel{
		styles:       styles,
		profile:      p,
		saveTarget:   saveTarget,
		groups:       groups,
		focusedGroup: 0,
		focusedField: 0,
		editing:      false,
	}
}
