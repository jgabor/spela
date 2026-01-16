package profile

type Preset string

const (
	PresetPerformance Preset = "performance"
	PresetBalanced    Preset = "balanced"
	PresetQuality     Preset = "quality"
	PresetCustom      Preset = "custom"
)

type DLSSMode string

const (
	DLSSModeOff              DLSSMode = "off"
	DLSSModeUltraPerformance DLSSMode = "ultra_performance"
	DLSSModePerformance      DLSSMode = "performance"
	DLSSModeBalanced         DLSSMode = "balanced"
	DLSSModeQuality          DLSSMode = "quality"
	DLSSModeDLAA             DLSSMode = "dlaa"
)

type DLSSPreset string

const (
	DLSSPresetDefault DLSSPreset = "default"
	DLSSPresetA       DLSSPreset = "A"
	DLSSPresetB       DLSSPreset = "B"
	DLSSPresetC       DLSSPreset = "C"
	DLSSPresetD       DLSSPreset = "D"
	DLSSPresetLatest  DLSSPreset = "latest"
)

type DLSSModelPreset string

const (
	DLSSModelPresetAuto DLSSModelPreset = "auto"
	DLSSModelPresetK    DLSSModelPreset = "k"
	DLSSModelPresetL    DLSSModelPreset = "l"
	DLSSModelPresetM    DLSSModelPreset = "m"
)

type Profile struct {
	Name   string `yaml:"name,omitempty"`
	Preset Preset `yaml:"preset"`

	DLSS     DLSSSettings     `yaml:"dlss,omitempty"`
	GPU      GPUSettings      `yaml:"gpu,omitempty"`
	CPU      CPUSettings      `yaml:"cpu,omitempty"`
	Proton   ProtonSettings   `yaml:"proton,omitempty"`
	Ludusavi LudusaviSettings `yaml:"ludusavi,omitempty"`
	Overlay  OverlaySettings  `yaml:"overlay,omitempty"`
}

type LudusaviSettings struct {
	BackupOnLaunch  bool `yaml:"backup_on_launch,omitempty"`
	RestoreOnLaunch bool `yaml:"restore_on_launch,omitempty"`
}

type OverlaySettings struct {
	Enabled       bool   `yaml:"enabled,omitempty"`
	Position      string `yaml:"position,omitempty"`
	ShowFPS       bool   `yaml:"show_fps,omitempty"`
	ShowFrametime bool   `yaml:"show_frametime,omitempty"`
	ShowCPU       bool   `yaml:"show_cpu,omitempty"`
	ShowGPU       bool   `yaml:"show_gpu,omitempty"`
	ShowVRAM      bool   `yaml:"show_vram,omitempty"`
	ToggleKey     string `yaml:"toggle_key,omitempty"`
}

type DLSSSettings struct {
	SRMode        DLSSMode        `yaml:"sr_mode,omitempty"`
	SRPreset      DLSSPreset      `yaml:"sr_preset,omitempty"`
	SRModelPreset DLSSModelPreset `yaml:"sr_model_preset,omitempty"`
	SROverride    bool            `yaml:"sr_override,omitempty"`
	RRMode        DLSSMode        `yaml:"rr_mode,omitempty"`
	RRPreset      DLSSPreset      `yaml:"rr_preset,omitempty"`
	RROverride    bool            `yaml:"rr_override,omitempty"`
	FGEnabled     bool            `yaml:"fg_enabled,omitempty"`
	FGOverride    bool            `yaml:"fg_override,omitempty"`
	MultiFrame    int             `yaml:"multi_frame,omitempty"`
	Indicator     bool            `yaml:"indicator,omitempty"`
	FGIndicator   bool            `yaml:"fg_indicator,omitempty"`
}

type GPUSettings struct {
	ShaderCache          bool   `yaml:"shader_cache,omitempty"`
	ShaderCachePath      string `yaml:"shader_cache_path,omitempty"`
	ThreadedOptimization bool   `yaml:"threaded_optimization,omitempty"`
	ClockOffset          int    `yaml:"clock_offset,omitempty"`
	MemoryOffset         int    `yaml:"memory_offset,omitempty"`
	PowerMizer           string `yaml:"power_mizer,omitempty"`
}

type CPUSettings struct {
	Governor string `yaml:"governor,omitempty"`
	SMT      *bool  `yaml:"smt,omitempty"`
	Affinity string `yaml:"affinity,omitempty"`
}

type ProtonSettings struct {
	EnableWayland    bool `yaml:"enable_wayland,omitempty"`
	EnableHDR        bool `yaml:"enable_hdr,omitempty"`
	EnableNGXUpdater bool `yaml:"enable_ngx_updater,omitempty"`
}

func DefaultPresets() map[Preset]*Profile {
	return map[Preset]*Profile{
		PresetPerformance: {
			Preset: PresetPerformance,
			DLSS: DLSSSettings{
				SRMode:        DLSSModeUltraPerformance,
				SRPreset:      DLSSPresetLatest,
				SRModelPreset: DLSSModelPresetAuto,
				SROverride:    true,
				FGEnabled:     true,
				FGOverride:    true,
				MultiFrame:    2,
			},
			GPU: GPUSettings{
				ShaderCache:          true,
				ThreadedOptimization: true,
			},
			Proton: ProtonSettings{
				EnableWayland: true,
			},
		},
		PresetBalanced: {
			Preset: PresetBalanced,
			DLSS: DLSSSettings{
				SRMode:        DLSSModeBalanced,
				SRPreset:      DLSSPresetLatest,
				SRModelPreset: DLSSModelPresetAuto,
				SROverride:    true,
				FGEnabled:     true,
				FGOverride:    true,
				MultiFrame:    1,
			},
			GPU: GPUSettings{
				ShaderCache:          true,
				ThreadedOptimization: true,
			},
			Proton: ProtonSettings{
				EnableWayland: true,
			},
		},
		PresetQuality: {
			Preset: PresetQuality,
			DLSS: DLSSSettings{
				SRMode:        DLSSModeDLAA,
				SRPreset:      DLSSPresetLatest,
				SRModelPreset: DLSSModelPresetAuto,
				SROverride:    true,
				FGEnabled:     false,
			},
			GPU: GPUSettings{
				ShaderCache:          true,
				ThreadedOptimization: true,
			},
			Proton: ProtonSettings{
				EnableWayland: true,
			},
		},
		PresetCustom: {
			Preset: PresetCustom,
			GPU: GPUSettings{
				ShaderCache: true,
			},
			Proton: ProtonSettings{
				EnableWayland: true,
			},
		},
	}
}

func FromPreset(preset Preset) *Profile {
	presets := DefaultPresets()
	if p, ok := presets[preset]; ok {
		copy := *p
		return &copy
	}
	return DefaultPresets()[PresetBalanced]
}
