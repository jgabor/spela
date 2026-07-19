export const srModeOptions = [
  { value: '', label: 'No mode' },
  { value: 'off', label: 'Off' },
  { value: 'ultra_performance', label: 'Ultra performance' },
  { value: 'performance', label: 'Performance' },
  { value: 'balanced', label: 'Balanced' },
  { value: 'quality', label: 'Quality' },
  { value: 'dlaa', label: 'DLAA' }
]

const modelPresetOptions = [
  { value: 'A', label: 'A' },
  { value: 'B', label: 'B' },
  { value: 'C', label: 'C' },
  { value: 'D', label: 'D' },
  { value: 'E', label: 'E' },
  { value: 'F', label: 'F' },
  { value: 'J', label: 'J' },
  { value: 'K', label: 'K' },
  { value: 'L', label: 'L' },
  { value: 'M', label: 'M' }
]

export const srPresetOptions = [
  { value: '', label: 'No preset' },
  { value: 'default', label: 'Driver default' },
  { value: 'auto', label: 'Auto (mode-linked)' },
  ...modelPresetOptions
]

export const rrPresetOptions = [
  { value: '', label: 'No preset' },
  { value: 'default', label: 'Driver default' },
  ...modelPresetOptions
]

export const multiFrameOptions = [
  { value: 0, label: '0 (off)' },
  { value: 1, label: '1' },
  { value: 2, label: '2' },
  { value: 3, label: '3' },
  { value: 4, label: '4' }
]

export const powerMizerOptions = [
  { value: '', label: 'No power mode' },
  { value: 'adaptive', label: 'Adaptive' },
  { value: 'max', label: 'Max performance' }
]

export const frameGenerationOptions = [
  { value: 'no_override', label: 'No override value' },
  { value: 'enabled', label: 'Enabled' },
  { value: 'disabled', label: 'Disabled' }
]

export const clockOffsetOptions = [
  { value: 0, label: '0 MHz' },
  { value: -200, label: '-200 MHz' },
  { value: -100, label: '-100 MHz' },
  { value: -50, label: '-50 MHz' },
  { value: 50, label: '+50 MHz' },
  { value: 100, label: '+100 MHz' },
  { value: 150, label: '+150 MHz' },
  { value: 200, label: '+200 MHz' },
  { value: 250, label: '+250 MHz' },
  { value: 300, label: '+300 MHz' }
]

export const memoryOffsetOptions = [
  { value: 0, label: '0 MHz' },
  { value: -500, label: '-500 MHz' },
  { value: -200, label: '-200 MHz' },
  { value: 200, label: '+200 MHz' },
  { value: 500, label: '+500 MHz' },
  { value: 750, label: '+750 MHz' },
  { value: 1000, label: '+1000 MHz' }
]

export const governorOptions = [
  { value: '', label: 'No governor' },
  { value: 'performance', label: 'Performance' },
  { value: 'powersave', label: 'Powersave' },
  { value: 'schedutil', label: 'Schedutil' },
  { value: 'ondemand', label: 'Ondemand' }
]

export const smtOptions = [
  { value: '', label: 'No SMT value' },
  { value: 'true', label: 'Enabled' },
  { value: 'false', label: 'Disabled' }
]

export const overlayPositionOptions = [
  { value: '', label: 'No position' },
  { value: 'top-left', label: 'Top left' },
  { value: 'top-right', label: 'Top right' },
  { value: 'bottom-left', label: 'Bottom left' },
  { value: 'bottom-right', label: 'Bottom right' }
]

// The maintained frontend projection contract. Layout remains in Svelte;
// persistence, empty fixtures, and patch generation share these names.
export const profileFieldDefinitions = [
  ['srMode', 'dlss.sr_mode', ''], ['srPreset', 'dlss.sr_preset', ''],
  ['srOverride', 'dlss.sr_override', false], ['rrMode', 'dlss.rr_mode', ''],
  ['rrPreset', 'dlss.rr_preset', ''], ['rrOverride', 'dlss.rr_override', false],
  ['fgEnabled', 'dlss.fg_enabled', false], ['fgOverride', 'dlss.fg_override', false],
  ['fgIndicator', 'dlss.fg_indicator', false], ['multiFrame', 'dlss.multi_frame', 0],
  ['indicator', 'dlss.indicator', false], ['shaderCache', 'gpu.shader_cache', false],
  ['shaderCachePath', 'gpu.shader_cache_path', ''],
  ['threadedOptimization', 'gpu.threaded_optimization', false],
  ['powerMizer', 'gpu.power_mizer', ''], ['clockOffset', 'gpu.clock_offset', 0],
  ['memoryOffset', 'gpu.memory_offset', 0], ['governor', 'cpu.governor', ''],
  ['smt', 'cpu.smt', ''], ['enableHdr', 'proton.enable_hdr', false],
  ['enableWayland', 'proton.enable_wayland', false],
  ['enableNgxUpdater', 'proton.enable_ngx_updater', false],
  ['vkd3dHeap', 'proton.vkd3d_heap', false], ['overlayEnabled', 'overlay.enabled', false],
  ['overlayPosition', 'overlay.position', ''], ['overlayShowFps', 'overlay.show_fps', false],
  ['overlayShowFrametime', 'overlay.show_frametime', false],
  ['overlayShowCpu', 'overlay.show_cpu', false], ['overlayShowGpu', 'overlay.show_gpu', false],
  ['overlayShowVram', 'overlay.show_vram', false], ['overlayToggleKey', 'overlay.toggle_key', '']
]

export const profilePropertyByField = Object.fromEntries(
  profileFieldDefinitions.map(([property, field]) => [field, property])
)
export const profileFieldByProperty = Object.fromEntries(
  profileFieldDefinitions.map(([property, field]) => [property, field])
)

export function emptyProfile() {
  return Object.assign(
    Object.fromEntries(profileFieldDefinitions.map(([property, , value]) => [property, value])),
    { inheritedFromDefault: false, semantics: [] }
  )
}
