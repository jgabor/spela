// Test data only. The real words come from the Go field descriptors through
// the profile projection (see profile_vocabulary_test.go); tests build
// projections with this helper so components are exercised with words that are
// not typed into the components themselves.
const labels = {
  'dlss.sr_mode': 'Super resolution',
  'dlss.sr_preset': 'Resolution preset',
  'dlss.sr_override': 'Resolution override',
  'dlss.rr_mode': 'Ray reconstruction',
  'dlss.rr_preset': 'Ray preset',
  'dlss.rr_override': 'Ray override',
  'dlss.fg_enabled': 'Frame generation',
  'dlss.fg_override': 'Generation override',
  'dlss.multi_frame': 'Multi-frame',
  'dlss.indicator': 'SR indicator',
  'dlss.fg_indicator': 'FG indicator',
  'gpu.clock_offset': 'Clock offset',
  'gpu.memory_offset': 'Memory offset',
  'gpu.power_limit': 'Power limit',
  'gpu.fan_speed': 'Fan speed',
  'cpu.affinity': 'Affinity',
  'gpu.power_mizer': 'Power mode',
  'gpu.shader_cache': 'Shader cache',
  'gpu.shader_cache_path': 'Shader cache path',
  'gpu.threaded_optimization': 'Threaded optimization',
  'gpu.vrr': 'KDE VRR',
  'cpu.governor': 'Governor',
  'cpu.smt': 'SMT',
  'proton.enable_hdr': 'HDR',
  'proton.enable_wayland': 'Wayland',
  'proton.enable_ngx_updater': 'NGX updater',
  'proton.vkd3d_heap': 'VKD3D heap',
  'overlay.enabled': 'Enabled',
  'overlay.position': 'Position',
  'overlay.show_fps': 'Show FPS',
  'overlay.show_frametime': 'Show frametime',
  'overlay.show_cpu': 'Show CPU',
  'overlay.show_gpu': 'Show GPU',
  'overlay.show_vram': 'Show VRAM',
  'overlay.toggle_key': 'Toggle key'
}

const units = { 'gpu.clock_offset': 'MHz', 'gpu.memory_offset': 'MHz', 'gpu.power_limit': 'W', 'gpu.fan_speed': '%' }
const valueTexts = { 'gpu.clock_offset': '100 MHz', 'gpu.memory_offset': '0 MHz', 'gpu.power_limit': '0 W', 'gpu.fan_speed': '0%', 'dlss.sr_mode': 'Quality', 'cpu.affinity': 'none', 'proton.enable_hdr': 'On' }

export function semantic(field, source = 'default', { impact = 'environment', restore = 'ephemeral_launch_environment', allGames = false } = {}) {
  const own = source === 'override'
  return {
    field,
    source,
    impact,
    restore,
    label: labels[field] ?? field,
    description: `About ${labels[field] ?? field}.`,
    origin: allGames ? (own ? 'set_here' : 'system_default') : own ? 'this_game' : 'all_games',
    sourceText: allGames ? (own ? 'set here' : 'system default') : own ? 'this game · All games: Off' : 'from All games',
    valueText: valueTexts[field] ?? 'Off',
    unit: units[field] ?? '',
    hint: units[field] ? `Whole number in ${units[field]}.` : '',
    ownValueSuffix: '',
    ownValueCount: 0,
    ownValueAction: '',
    action: allGames ? (own ? 'Use system default' : 'Set here') : own ? 'Follow All games' : 'Set for this game'
  }
}

// allOverride marks every field as set on the page, for All games fixtures.
export const allOverride = () => Object.fromEntries(Object.keys(labels).map(field => [field, 'override']))

const systemState = new Set(['gpu.clock_offset', 'gpu.memory_offset', 'gpu.power_limit', 'gpu.fan_speed', 'gpu.vrr', 'cpu.governor', 'cpu.smt'])

// fieldSemantics projects every field. sources maps a field to 'override' for
// the ones the game sets itself; allGames builds the All games page instead.
export function fieldSemantics(sources = {}, { allGames = false } = {}) {
  return Object.keys(labels).map(field => semantic(field, sources[field] ?? 'default', {
    impact: systemState.has(field) ? 'system_state' : 'environment',
    restore: systemState.has(field) ? 'restorable_mutation' : 'ephemeral_launch_environment',
    allGames
  }))
}
