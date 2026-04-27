import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
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
  CheckDLLUpdates: vi.fn().mockResolvedValue([{ dllType: 'dlss', currentVersion: '3.7.0', latestVersion: '3.8.10', hasUpdate: true }]),
  GetDefaultProfile: vi.fn().mockResolvedValue(null),
  GetGame: vi.fn().mockResolvedValue({ appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] }),
  GetProfile: vi.fn().mockResolvedValue(fixtures.profile),
  HasDLLBackup: vi.fn().mockResolvedValue(true),
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
import { LaunchGame, SaveProfile, UpdateDLLs } from '../../wailsjs/go/gui/App'

describe('GameDetail current behavior', () => {
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

  it('renders DLL state and keeps failed DLL actions in a dismissible error banner', async () => {
    UpdateDLLs.mockRejectedValueOnce(new Error('download failed'))

    render(GameDetail, {
      props: {
        game: {
          appId: 1091500,
          name: 'Cyberpunk 2077',
          installDir: '/games/cyberpunk',
          dlls: [{ dllType: 'dlss', version: '3.7.0' }]
        },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('3.7.0')).toBeTruthy())
    expect(screen.getByText('Update all DLLs')).toBeTruthy()

    await fireEvent.click(screen.getByText('Update all DLLs'))

    await waitFor(() => expect(screen.getByText('Failed to update: download failed')).toBeTruthy())
    expect(screen.getByText('Failed to update: download failed')).toBeTruthy()

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Failed to update: download failed')).toBeNull()
  })

  it('shows launch guidance as an error and does not show launch success when direct launch is rejected', async () => {
    LaunchGame.mockRejectedValueOnce(new Error('Steam wrapper required: add spela %command% to the launch options.'))

    render(GameDetail, {
      props: {
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('▶ Launch')).toBeTruthy())
    await fireEvent.click(screen.getByText('▶ Launch'))

    await waitFor(() => {
      expect(screen.getByText('Failed to launch: Steam wrapper required: add spela %command% to the launch options.')).toBeTruthy()
    })
    expect(screen.queryByText('Game launched!')).toBeNull()
  })

  it('keeps profile save failures visible until dismissed', async () => {
    SaveProfile.mockRejectedValueOnce(new Error('permission denied'))

    render(GameDetail, {
      props: {
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByText('Save profile'))

    await waitFor(() => expect(screen.getByText('Failed to save: permission denied')).toBeTruthy())

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Failed to save: permission denied')).toBeNull()
  })
})
