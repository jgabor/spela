import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const fixtures = vi.hoisted(() => ({
  profile: {
    srMode: 'quality',
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
      { field: 'proton.vkd3d_heap', source: 'override', impact: 'compatibility', restore: 'ephemeral_launch_environment' },
      { field: 'dlss.fg_enabled', source: 'default', impact: 'environment', restore: 'ephemeral_launch_environment' },
      { field: 'dlss.fg_override', source: 'default', impact: 'environment', restore: 'ephemeral_launch_environment' }
    ]
  },
  defaultProfile: {
    srMode: 'balanced',
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
import {
  clockOffsetOptions, frameGenerationOptions, multiFrameOptions, smtOptions, srModeOptions
} from './profileFieldOptions.js'

function desktop(overrides = {}) {
  return {
    CheckDLLUpdates: vi.fn().mockResolvedValue([{ dllType: 'dlss', currentVersion: '3.7.0', latestVersion: '3.8.10', hasUpdate: true }]),
    GetDefaultProfile: vi.fn().mockResolvedValue(null),
    GetGame: vi.fn().mockResolvedValue({ appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] }),
    GetProfile: vi.fn().mockImplementation(async () => structuredClone(fixtures.profile)),
    HasDLLBackup: vi.fn().mockResolvedValue(true),
    InstallDLL: vi.fn().mockResolvedValue(undefined),
    LaunchGame: vi.fn().mockResolvedValue(undefined),
    ListDLLInstallTypes: vi.fn().mockResolvedValue([]),
    ListDLLVersions: vi.fn().mockResolvedValue([]),
    RestoreDLLs: vi.fn().mockResolvedValue(undefined),
    PatchDefaultProfile: vi.fn().mockResolvedValue(undefined),
    PatchProfile: vi.fn().mockResolvedValue(undefined),
    SubscribeDLLProgress: vi.fn((handler) => progressEvents.subscribe(handler)),
    UpdateDLLs: vi.fn().mockResolvedValue({ updated: 1, unchanged: 0, failed: 0, failures: [] }),
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
      expect(screen.getByText('All games (default)')).toBeTruthy()
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
        profileMode: 'game',
        aspect: 'dlls'
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

  it('reports an all-no-op update without claiming files were updated', async () => {
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn().mockResolvedValue({ updated: 0, unchanged: 1 }),
      GetGame: vi.fn().mockResolvedValue({
        appId: 1091500,
        name: 'Cyberpunk 2077',
        installDir: '/games/cyberpunk',
        dlls: [{ dllType: 'dlss', version: '3.7.0' }]
      })
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
        profileMode: 'game',
        aspect: 'dlls'
      }
    })

    await fireEvent.click(await screen.findByText('Update all DLLs'))
    await waitFor(() => expect(screen.getByText('DLLs already up to date')).toBeTruthy())
    expect(screen.queryByText('DLLs updated!')).toBeNull()
  })

  it('reports successful and failed items from one resolved batch outcome', async () => {
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn().mockResolvedValue({
        updated: 1,
        unchanged: 0,
        failed: 1,
        failures: [{ path: '/game/nvngx_dlssg.dll', error: 'checksum mismatch' }]
      })
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
        profileMode: 'game',
        aspect: 'dlls'
      }
    })

    await fireEvent.click(await screen.findByText('Update all DLLs'))
    await waitFor(() => expect(screen.getByText(/1 updated, 0 current, 1 failed/)).toBeTruthy())
    expect(screen.getByText(/nvngx_dlssg\.dll: checksum mismatch/)).toBeTruthy()
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
        profileMode: 'game',
        aspect: 'dlls'
      }
    })

    await waitFor(() => expect(screen.getByText('Update all DLLs')).toBeTruthy())
    await fireEvent.click(screen.getByText('Update all DLLs'))
    expect(screen.getByText('Install DLL').disabled).toBe(true)
    expect(screen.getByText('Restore original DLLs').disabled).toBe(true)
    progressEvents.emit('downloading')

    await waitFor(() => expect(screen.getByText('downloading…')).toBeTruthy())
    firstUpdate.resolve({ updated: 1, unchanged: 0, failed: 0, failures: [] })
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
        profileMode: 'game',
        aspect: 'dlls'
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

  it('shows Steam wrapper launch guidance instead of a launch button', async () => {
    render(GameDetail, {
      props: {
        desktop: desktop(),
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => {
      expect(screen.getByText(/Launch via Steam/)).toBeTruthy()
      expect(screen.queryByText('▶ Launch')).toBeNull()
    })
  })

  it('keeps profile save failures visible until dismissed', async () => {
    const replacementDesktop = desktop({
      PatchProfile: vi.fn().mockRejectedValueOnce(new Error('permission denied'))
    })

    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
        profileMode: 'game'
      }
    })

    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await fireEvent.click(screen.getByText('Save profile'))

    await waitFor(() => expect(screen.getByText('Failed to save: permission denied')).toBeTruthy())

    vi.useFakeTimers()
    vi.advanceTimersByTime(5000)
    expect(screen.getByText('Failed to save: permission denied')).toBeTruthy()
    vi.useRealTimers()

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Failed to save: permission denied')).toBeNull()
  })

  it('queues inherited pin and overridden boolean reset actions', async () => {
    const replacementDesktop = desktop()
    render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Pin' }))
    const heapField = screen.getByLabelText('VKD3D Heap').closest('.field')
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Reset' }))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.enable_hdr', operation: 'pin' },
      { field: 'proton.vkd3d_heap', operation: 'reset' }
    ]))
  })

  it('uses explicit labels rather than calling persisted zero values defaults', () => {
    const labels = [...clockOffsetOptions, ...frameGenerationOptions, ...multiFrameOptions, ...smtOptions, ...srModeOptions]
      .map(option => option.label)
    expect(labels).not.toContain('(default)')
    expect(labels).toEqual(expect.arrayContaining(['0 MHz', '0 (off)', 'No SMT value', 'No mode', 'No override value']))
  })

  it('toggles pending actions and lets a later edit win over reset', async () => {
    const replacementDesktop = desktop()
    render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    const heapField = screen.getByLabelText('VKD3D Heap').closest('.field')
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Reset' }))
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Cancel Reset' }))
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Reset' }))
    await fireEvent.click(screen.getByLabelText('VKD3D Heap'))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.vkd3d_heap', operation: 'set', value: true }
    ]))
  })

  it('pins and resets both frame generation backing fields as one action', async () => {
    const replacementDesktop = desktop()
    const view = render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    const frameField = screen.getByText('Frame generation').closest('.field')
    await fireEvent.click(within(frameField).getByRole('button', { name: 'Pin' }))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'dlss.fg_enabled', operation: 'pin' },
      { field: 'dlss.fg_override', operation: 'pin' }
    ]))

    const overridden = structuredClone(fixtures.profile)
    overridden.fgEnabled = true
    overridden.fgOverride = true
    overridden.semantics = overridden.semantics.map(item =>
      item.field.startsWith('dlss.fg_') ? { ...item, source: 'override' } : item
    )
    replacementDesktop.GetProfile.mockResolvedValue(overridden)
    replacementDesktop.PatchProfile.mockClear()
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    const overriddenFrame = screen.getByText('Frame generation').closest('.field')
    await waitFor(() => expect(within(overriddenFrame).getByRole('button', { name: 'Reset' })).toBeTruthy())
    await fireEvent.click(within(overriddenFrame).getByRole('button', { name: 'Reset' }))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(292030, [
      { field: 'dlss.fg_enabled', operation: 'reset' },
      { field: 'dlss.fg_override', operation: 'reset' }
    ]))
  })

  it('sends false and numeric zero as explicit sets in one atomic batch', async () => {
    const replacementDesktop = desktop()
    render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    const clockField = screen.getByText('Clock offset').closest('.field')
    await fireEvent.click(clockField.querySelector('.trigger'))
    await fireEvent.click(within(clockField).getByRole('button', { name: '0 MHz' }))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'gpu.clock_offset', operation: 'set', value: 0 },
      { field: 'proton.enable_hdr', operation: 'set', value: false }
    ]))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1)
  })

  it('sends optional SMT nil representation as an explicit set', async () => {
    const profile = structuredClone(fixtures.profile)
    profile.smt = 'true'
    const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(profile) })
    render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    const smtField = screen.getByText('SMT', { selector: 'label' }).closest('.field')
    await fireEvent.click(smtField.querySelector('.trigger'))
    await fireEvent.click(within(smtField).getByRole('button', { name: 'No SMT value' }))
    await fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'cpu.smt', operation: 'set', value: '' }
    ]))
  })

  it('offers explicit unset for the defaults root', async () => {
    const replacementDesktop = desktop({ GetDefaultProfile: vi.fn().mockResolvedValue(structuredClone(fixtures.defaultProfile)) })
    render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
    await waitFor(() => expect(screen.getByText('Save default profile')).toBeTruthy())
    await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Unset' }))
    await fireEvent.click(screen.getByText('Save default profile'))
    await waitFor(() => expect(replacementDesktop.PatchDefaultProfile).toHaveBeenCalledWith([
      { field: 'proton.enable_hdr', operation: 'reset' }
    ]))
  })

  it('snapshots the target app before awaiting the atomic save', async () => {
    const pending = deferred()
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockReturnValue(pending.promise) })
    const view = render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    void fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, expect.any(Array)))
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    pending.resolve(structuredClone(fixtures.profile))
    await waitFor(() => expect(replacementDesktop.GetProfile).toHaveBeenCalledWith(292030))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1)
    expect(replacementDesktop.GetGame).not.toHaveBeenCalledWith(1091500)
    expect(screen.queryByText('Profile saved!')).toBeNull()
  })

  it('does not show a stale save error after navigating away', async () => {
    const pending = deferred()
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockReturnValue(pending.promise) })
    const view = render(GameDetail, { props: {
      desktop: replacementDesktop,
      game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
      profileMode: 'game'
    } })
    await waitFor(() => expect(screen.getByText('Save profile')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    void fireEvent.click(screen.getByText('Save profile'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalled())
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    pending.reject(new Error('old target failed'))
    await waitFor(() => expect(replacementDesktop.GetProfile).toHaveBeenCalledWith(292030))
    expect(screen.queryByText('Failed to save: old target failed')).toBeNull()
  })
})
