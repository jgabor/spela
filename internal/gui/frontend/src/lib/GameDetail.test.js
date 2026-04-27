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
  },
  defaultProfile: {
    srMode: 'balanced',
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
    clockOffset: 50,
    memoryOffset: 0,
    governor: '',
    smt: '',
    enableHdr: true,
    enableWayland: false,
    enableNgxUpdater: false,
    vkd3dHeap: false,
    inheritedFromDefault: false,
    semantics: [
      { field: 'dlss.sr_mode', source: 'default', impact: 'environment', restore: 'ephemeral_launch_environment' },
      { field: 'gpu.clock_offset', source: 'default', impact: 'system_state', restore: 'restorable_mutation' },
      { field: 'proton.enable_hdr', source: 'default', impact: 'compatibility', restore: 'ephemeral_launch_environment' }
    ]
  }
}))

const progressEvents = vi.hoisted(() => ({
  listeners: new Set(),
  subscribe(handler) {
    this.listeners.add(handler)
    return () => this.listeners.delete(handler)
  },
  emit(stage) {
    for (const listener of this.listeners) {
      listener(stage)
    }
  },
  reset() {
    this.listeners.clear()
  }
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn((event, handler) => event === 'dll:progress' ? progressEvents.subscribe(handler) : () => {}),
  Quit: vi.fn()
}))

import GameDetail from './GameDetail.svelte'

function desktop(overrides = {}) {
  return {
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
    SubscribeDLLProgress: vi.fn((handler) => progressEvents.subscribe(handler)),
    UpdateDLLs: vi.fn().mockResolvedValue(undefined),
    VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue(''),
    ...overrides
  }
}

describe('GameDetail current behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    progressEvents.reset()
  })

  function deferred() {
    let resolve
    let reject
    const promise = new Promise((nextResolve, nextReject) => {
      resolve = nextResolve
      reject = nextReject
    })
    return { promise, resolve, reject }
  }

  it('renders effective values and inherited intent for game and default profiles', async () => {
    const gameProfile = {
      ...fixtures.profile,
      inheritedFromDefault: true
    }
    const replacementDesktop = desktop({
      GetDefaultProfile: vi.fn().mockResolvedValue(fixtures.defaultProfile),
      GetProfile: vi.fn().mockResolvedValue(gameProfile)
    })
    const { unmount } = render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => {
      expect(screen.getByText('Using default profile values.')).toBeTruthy()
      expect(screen.getByText('Quality')).toBeTruthy()
      expect(screen.getByText('+100 MHz')).toBeTruthy()
      expect(screen.getByLabelText('HDR').checked).toBe(true)
      expect(screen.getByText('source override · impact environment · restore ephemeral_launch_environment')).toBeTruthy()
      expect(screen.getByText('source default · impact system_state · restore restorable_mutation')).toBeTruthy()
      expect(screen.getByText('source default · impact compatibility · restore ephemeral_launch_environment')).toBeTruthy()
    })

    unmount()
    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        profileMode: 'default'
      }
    })

    await waitFor(() => {
      expect(screen.getByText('Default profile')).toBeTruthy()
      expect(screen.getByText('Balanced')).toBeTruthy()
      expect(screen.getByText('+50 MHz')).toBeTruthy()
      expect(screen.getByLabelText('HDR').checked).toBe(true)
      expect(screen.getByText('impact environment · restore ephemeral_launch_environment')).toBeTruthy()
      expect(screen.getByText('impact system_state · restore restorable_mutation')).toBeTruthy()
      expect(screen.queryByText('source default · impact environment · restore ephemeral_launch_environment')).toBeNull()
    })
  })

  it('renders DLL state and keeps failed DLL actions in a dismissible error banner', async () => {
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn().mockRejectedValueOnce(new Error('download failed'))
    })

    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
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

    vi.useFakeTimers()
    vi.advanceTimersByTime(5000)
    expect(screen.getByText('Failed to update: download failed')).toBeTruthy()
    vi.useRealTimers()

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Failed to update: download failed')).toBeNull()
  })

  it('shows active DLL progress and clears stale progress after success or failure', async () => {
    const firstUpdate = deferred()
    const secondUpdate = deferred()
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn()
        .mockReturnValueOnce(firstUpdate.promise)
        .mockReturnValueOnce(secondUpdate.promise)
    })

    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        game: {
          appId: 1091500,
          name: 'Cyberpunk 2077',
          installDir: '/games/cyberpunk',
          dlls: [{ dllType: 'dlss', version: '3.7.0' }]
        },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('Update all DLLs')).toBeTruthy())
    await fireEvent.click(screen.getByText('Update all DLLs'))
    progressEvents.emit('downloading')

    await waitFor(() => expect(screen.getByText('downloading…')).toBeTruthy())
    firstUpdate.resolve()
    await waitFor(() => expect(screen.queryByText('downloading…')).toBeNull())

    await fireEvent.click(screen.getByText('Update all DLLs'))
    progressEvents.emit('applying')

    await waitFor(() => expect(screen.getByText('applying…')).toBeTruthy())
    secondUpdate.reject(new Error('download failed'))
    await waitFor(() => expect(screen.queryByText('applying…')).toBeNull())
    expect(screen.getByText('Failed to update: download failed')).toBeTruthy()
  })

  it('unsubscribes from DLL progress when the detail view unmounts', async () => {
    const update = deferred()
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn().mockReturnValue(update.promise)
    })

    const { unmount } = render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        game: {
          appId: 1091500,
          name: 'Cyberpunk 2077',
          installDir: '/games/cyberpunk',
          dlls: [{ dllType: 'dlss', version: '3.7.0' }]
        },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('Update all DLLs')).toBeTruthy())
    await fireEvent.click(screen.getByText('Update all DLLs'))
    unmount()
    progressEvents.emit('still running')
    update.resolve()

    expect(replacementDesktop.SubscribeDLLProgress).toHaveBeenCalledTimes(1)
    expect(progressEvents.listeners.size).toBe(0)
  })

  it('shows launch guidance as an error and does not show launch success when direct launch is rejected', async () => {
    const replacementDesktop = desktop({
      LaunchGame: vi.fn().mockRejectedValueOnce(new Error('Steam wrapper required: add spela %command% to the launch options.'))
    })

    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
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
    const replacementDesktop = desktop({
      SaveProfile: vi.fn().mockRejectedValueOnce(new Error('permission denied'))
    })

    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByText('Save profile'))

    await waitFor(() => expect(screen.getByText('Failed to save: permission denied')).toBeTruthy())

    vi.useFakeTimers()
    vi.advanceTimersByTime(5000)
    expect(screen.getByText('Failed to save: permission denied')).toBeTruthy()
    vi.useRealTimers()

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Failed to save: permission denied')).toBeNull()
  })
})
