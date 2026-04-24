import { render, screen, waitFor } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const fixtures = vi.hoisted(() => ({
  profile: {
    srMode: 'quality',
    srPreset: '',
    srModelPreset: '',
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
    clockOffset: 100,
    memoryOffset: 0,
    governor: '',
    smt: '',
    enableHdr: true,
    enableWayland: false,
    enableNgxUpdater: false,
    vkd3dHeap: false,
    inheritedFromDefault: false,
    semantics: [
      { field: 'dlss.sr_mode', source: 'override', impact: 'environment', restore: 'ephemeral_launch_environment' },
      { field: 'gpu.clock_offset', source: 'default', impact: 'system_state', restore: 'restorable_mutation' },
      { field: 'proton.enable_hdr', source: 'default', impact: 'compatibility', restore: 'ephemeral_launch_environment' },
      { field: 'proton.vkd3d_heap', source: 'override', impact: 'compatibility', restore: 'ephemeral_launch_environment' }
    ]
  }
}))

vi.mock('../../wailsjs/go/gui/App', () => ({
  CheckDLLUpdates: vi.fn().mockResolvedValue([]),
  GetDefaultProfile: vi.fn().mockResolvedValue(null),
  GetGame: vi.fn().mockResolvedValue(null),
  GetProfile: vi.fn().mockResolvedValue(fixtures.profile),
  HasDLLBackup: vi.fn().mockResolvedValue(false),
  InstallDLL: vi.fn().mockResolvedValue(undefined),
  LaunchGame: vi.fn().mockResolvedValue(undefined),
  ListDLLInstallTypes: vi.fn().mockResolvedValue([]),
  ListDLLVersions: vi.fn().mockResolvedValue([]),
  RestoreDLLs: vi.fn().mockResolvedValue(undefined),
  SaveDefaultProfile: vi.fn().mockResolvedValue(undefined),
  SaveProfile: vi.fn().mockResolvedValue(undefined),
  UpdateDLLs: vi.fn().mockResolvedValue(undefined),
  VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue('')
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn().mockReturnValue(() => {})
}))

import GameDetail from './GameDetail.svelte'

describe('GameDetail profile semantics', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders shared source impact and restore semantics for profile fields', async () => {
    render(GameDetail, {
      props: {
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => {
      expect(screen.getByText('source override · impact environment · restore ephemeral_launch_environment')).toBeTruthy()
      expect(screen.getByText('source default · impact system_state · restore restorable_mutation')).toBeTruthy()
      expect(screen.getByText('source default · impact compatibility · restore ephemeral_launch_environment')).toBeTruthy()
    })
  })
})
