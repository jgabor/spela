package profile

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
	DLSSPresetAuto    DLSSPreset = "auto"
	DLSSPresetA       DLSSPreset = "A"
	DLSSPresetB       DLSSPreset = "B"
	DLSSPresetC       DLSSPreset = "C"
	DLSSPresetD       DLSSPreset = "D"
	DLSSPresetE       DLSSPreset = "E"
	DLSSPresetF       DLSSPreset = "F"
	DLSSPresetJ       DLSSPreset = "J"
	DLSSPresetK       DLSSPreset = "K"
	DLSSPresetL       DLSSPreset = "L"
	DLSSPresetM       DLSSPreset = "M"
)

type Profile struct {
	Name string `yaml:"name,omitempty"`

	DLSS    DLSSSettings    `yaml:"dlss,omitempty"`
	GPU     GPUSettings     `yaml:"gpu,omitempty"`
	CPU     CPUSettings     `yaml:"cpu,omitempty"`
	Proton  ProtonSettings  `yaml:"proton,omitempty"`
	Overlay OverlaySettings `yaml:"overlay,omitempty"`

	// Overrides tracks which leaf fields are explicitly pinned on this
	// profile (true) as opposed to inheriting from the defaults profile.
	// Keys are dot-path field keys defined in inheritance.go (e.g.
	// "proton.vkd3d_heap"). A profile whose Overrides is nil on load is
	// treated as legacy and run through migrateInheritance to reconstruct
	// the map from the stored values.
	Overrides map[string]bool `yaml:"overrides,omitempty"`
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
	SRMode      DLSSMode   `yaml:"sr_mode,omitempty"`
	SRPreset    DLSSPreset `yaml:"sr_preset,omitempty"`
	SROverride  bool       `yaml:"sr_override,omitempty"`
	RRMode      DLSSMode   `yaml:"rr_mode,omitempty"`
	RRPreset    DLSSPreset `yaml:"rr_preset,omitempty"`
	RROverride  bool       `yaml:"rr_override,omitempty"`
	FGEnabled   bool       `yaml:"fg_enabled,omitempty"`
	FGOverride  bool       `yaml:"fg_override,omitempty"`
	MultiFrame  int        `yaml:"multi_frame,omitempty"`
	Indicator   bool       `yaml:"indicator,omitempty"`
	FGIndicator bool       `yaml:"fg_indicator,omitempty"`
}

type GPUSettings struct {
	ShaderCache          bool   `yaml:"shader_cache,omitempty"`
	ShaderCachePath      string `yaml:"shader_cache_path,omitempty"`
	ThreadedOptimization bool   `yaml:"threaded_optimization,omitempty"`
	ClockOffset          int    `yaml:"clock_offset,omitempty"`
	MemoryOffset         int    `yaml:"memory_offset,omitempty"`
	PowerLimit           int    `yaml:"power_limit,omitempty"`
	PowerMizer           string `yaml:"power_mizer,omitempty"`
	FanSpeed             int    `yaml:"fan_speed,omitempty"`
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
	VKD3DHeap        bool `yaml:"vkd3d_heap,omitempty"`
}

// Clone returns an independent profile copy, including pointer and override state.
func (p *Profile) Clone() *Profile {
	if p == nil {
		return &Profile{}
	}
	clone := *p
	if p.CPU.SMT != nil {
		value := *p.CPU.SMT
		clone.CPU.SMT = &value
	}
	clone.Overrides = make(map[string]bool, len(p.Overrides))
	for field, overridden := range p.Overrides {
		clone.Overrides[field] = overridden
	}
	if p.Overrides == nil {
		clone.Overrides = nil
	}
	return &clone
}
