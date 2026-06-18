export const srModeOptions = [
  { value: '', label: '(default)' },
  { value: 'off', label: 'Off' },
  { value: 'ultra_performance', label: 'Ultra performance' },
  { value: 'performance', label: 'Performance' },
  { value: 'balanced', label: 'Balanced' },
  { value: 'quality', label: 'Quality' },
  { value: 'dlaa', label: 'DLAA' }
]

export const srPresetOptions = [
  { value: '', label: '(default)' },
  { value: 'auto', label: 'Auto (mode-linked)' },
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

export const multiFrameOptions = [
  { value: 0, label: '(default)' },
  { value: 1, label: '1' },
  { value: 2, label: '2' },
  { value: 3, label: '3' },
  { value: 4, label: '4' }
]

export const powerMizerOptions = [
  { value: '', label: '(default)' },
  { value: 'adaptive', label: 'Adaptive' },
  { value: 'max', label: 'Max performance' }
]

export const frameGenerationOptions = [
  { value: '(default)', label: '(default)' },
  { value: 'true', label: 'true' },
  { value: 'false', label: 'false' }
]

export const clockOffsetOptions = [
  { value: 0, label: '(default)' },
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
  { value: 0, label: '(default)' },
  { value: -500, label: '-500 MHz' },
  { value: -200, label: '-200 MHz' },
  { value: 200, label: '+200 MHz' },
  { value: 500, label: '+500 MHz' },
  { value: 750, label: '+750 MHz' },
  { value: 1000, label: '+1000 MHz' }
]

export const governorOptions = [
  { value: '', label: '(default)' },
  { value: 'performance', label: 'Performance' },
  { value: 'powersave', label: 'Powersave' },
  { value: 'schedutil', label: 'Schedutil' },
  { value: 'ondemand', label: 'Ondemand' }
]

export const smtOptions = [
  { value: '', label: '(default)' },
  { value: 'true', label: 'Enabled' },
  { value: 'false', label: 'Disabled' }
]

export const overlayPositionOptions = [
  { value: '', label: '(default)' },
  { value: 'top-left', label: 'Top left' },
  { value: 'top-right', label: 'Top right' },
  { value: 'bottom-left', label: 'Bottom left' },
  { value: 'bottom-right', label: 'Bottom right' }
]

export function emptyProfile() {
  return {
    srMode: '',
    srPreset: '',
    srOverride: false,
    rrMode: '',
    rrPreset: '',
    rrOverride: false,
    fgEnabled: false,
    fgOverride: false,
    fgIndicator: false,
    multiFrame: 0,
    indicator: false,
    shaderCache: false,
    shaderCachePath: '',
    threadedOptimization: false,
    powerMizer: '',
    clockOffset: 0,
    memoryOffset: 0,
    governor: '',
    smt: '',
    enableHdr: false,
    enableWayland: false,
    enableNgxUpdater: false,
    vkd3dHeap: false,
    overlayEnabled: false,
    overlayPosition: '',
    overlayShowFps: false,
    overlayShowFrametime: false,
    overlayShowCpu: false,
    overlayShowGpu: false,
    overlayShowVram: false,
    overlayToggleKey: '',
    backupOnLaunch: false,
    inheritedFromDefault: false
  }
}
