import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const helper = await vi.hoisted(() => import('./testSemantics.js'))

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
    semantics: helper.fieldSemantics({ 'dlss.sr_mode': 'override', 'proton.vkd3d_heap': 'override' })
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
    semantics: helper.fieldSemantics(helper.allOverride(), { allGames: true })
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
  frameGenerationOptions, multiFrameOptions, smtOptions, srModeOptions
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
    ApplyToGamesWithOwnValue: vi.fn().mockResolvedValue(undefined),
    PatchDefaultProfile: vi.fn().mockResolvedValue(undefined),
    PatchProfile: vi.fn().mockResolvedValue(undefined),
    PreviewApplyToGamesWithOwnValue: vi.fn().mockResolvedValue(undefined),
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
      expect(screen.getByText('This game has no profile of its own; every value follows All games.')).toBeTruthy()
      expect(screen.getByText('Quality')).toBeTruthy()
      expect(screen.getByLabelText('Clock offset').value).toBe('100')
      expect(screen.getByLabelText('HDR').checked).toBe(true)
      expect(screen.getAllByText('this game · All games: Off').length).toBeGreaterThan(0)
      expect(screen.getAllByText('from All games').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Follow All games', { selector: 'button' }).length).toBeGreaterThan(0)
      expect(screen.getAllByText('Set for this game', { selector: 'button' }).length).toBeGreaterThan(0)
    })

    unmount()
    render(GameDetail, {
      props: {
        desktop: replacementDesktop,
        profileMode: 'default'
      }
    })

    await waitFor(() => {
      expect(screen.getByText('All games')).toBeTruthy()
      expect(screen.getByText('Balanced')).toBeTruthy()
      expect(screen.getByLabelText('Clock offset').value).toBe('50')
      expect(screen.getByLabelText('HDR').checked).toBe(true)
      expect(screen.getAllByText('set here').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Use system default', { selector: 'button' }).length).toBeGreaterThan(0)
      expect(screen.queryByText('Set for this game', { selector: 'button' })).toBeNull()
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

  it('shows the Game block with Steam wrapper guidance on the DLLs tab only', async () => {
    const game = { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', prefixPath: '/prefix', dlls: [] }
    const view = render(GameDetail, { props: { desktop: desktop(), game, profileMode: 'game', aspect: 'profile' } })
    await waitFor(() => expect(screen.getByText('Cyberpunk 2077')).toBeTruthy())
    expect(screen.queryByText(/Launch via Steam/)).toBeNull()
    expect(screen.queryByText('/games/cyberpunk')).toBeNull()

    await view.rerender({ desktop: desktop(), game, profileMode: 'game', aspect: 'dlls' })
    await waitFor(() => {
      expect(screen.getByText(/Launch via Steam/)).toBeTruthy()
      expect(screen.getByText('/games/cyberpunk')).toBeTruthy()
      expect(screen.getByText('/prefix')).toBeTruthy()
      expect(screen.queryByText('▶ Launch')).toBeNull()
    })
  })

  const cyberpunk = { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] }

  function renderGame(replacementDesktop, game = cyberpunk) {
    return render(GameDetail, { props: { desktop: replacementDesktop, game, profileMode: 'game' } })
  }

  function updatedProfile(changes = {}, sources = { 'dlss.sr_mode': 'override', 'proton.vkd3d_heap': 'override' }) {
    return { ...structuredClone(fixtures.profile), ...changes, semantics: helper.fieldSemantics(sources) }
  }

  it('has no Save button: every control saves as it changes', async () => {
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockResolvedValue(updatedProfile({ enableHdr: false })) })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    expect(screen.queryByText('Save profile')).toBeNull()
    expect(screen.getByText('Saved at once')).toBeTruthy()

    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.enable_hdr', operation: 'set', value: false }
    ]))
    await waitFor(() => expect(screen.getByText(/Saved: HDR/)).toBeTruthy())
    expect(screen.getByText('Saved at once')).toBeTruthy()
  })

  it('says Saving while a write runs and Saved when it lands', async () => {
    const pending = deferred()
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockReturnValue(pending.promise) })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(screen.getByText('Saving…')).toBeTruthy())
    pending.resolve(updatedProfile({ enableHdr: false }))
    await waitFor(() => expect(screen.getByText('Saved at once')).toBeTruthy())
    expect(screen.getByText(/Saved: HDR/)).toBeTruthy()
  })

  it('returns the control to the stored value and keeps the failure visible until dismissed', async () => {
    const replacementDesktop = desktop({
      PatchProfile: vi.fn().mockRejectedValueOnce(new Error('permission denied'))
    })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    expect(screen.getByLabelText('HDR').checked).toBe(true)
    await fireEvent.click(screen.getByLabelText('HDR'))

    await waitFor(() => expect(screen.getAllByText('Not saved: permission denied').length).toBeGreaterThan(0))
    expect(screen.getByLabelText('HDR').checked).toBe(true)

    vi.useFakeTimers()
    vi.advanceTimersByTime(5000)
    expect(screen.getAllByText('Not saved: permission denied').length).toBeGreaterThan(0)
    vi.useRealTimers()

    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Dismiss')).toBeNull()
  })

  it('sends Set for this game as a pin and Follow All games as a reset, at once', async () => {
    const replacementDesktop = desktop()
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Set for this game' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.enable_hdr', operation: 'pin' }
    ]))
    const heapField = screen.getByLabelText('VKD3D heap').closest('.field')
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Follow All games' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.vkd3d_heap', operation: 'reset' }
    ]))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(2)
  })

  it('puts the remembered value back when Set for this game follows a Follow', async () => {
    const own = updatedProfile({ vkd3dHeap: true })
    const followed = updatedProfile({ vkd3dHeap: false }, { 'dlss.sr_mode': 'override' })
    const replacementDesktop = desktop({
      GetProfile: vi.fn().mockResolvedValue(own),
      PatchProfile: vi.fn().mockResolvedValueOnce(followed).mockResolvedValueOnce(own)
    })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('VKD3D heap')).toBeTruthy())
    expect(screen.getByLabelText('VKD3D heap').checked).toBe(true)
    await fireEvent.click(within(screen.getByLabelText('VKD3D heap').closest('.field')).getByRole('button', { name: 'Follow All games' }))
    await waitFor(() => expect(screen.getByLabelText('VKD3D heap').checked).toBe(false))
    await fireEvent.click(within(screen.getByLabelText('VKD3D heap').closest('.field')).getByRole('button', { name: 'Set for this game' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenLastCalledWith(1091500, [
      { field: 'proton.vkd3d_heap', operation: 'set', value: true }
    ]))
    await waitFor(() => expect(screen.getByLabelText('VKD3D heap').checked).toBe(true))
  })

  it('uses explicit labels rather than calling persisted zero values defaults', () => {
    const labels = [...frameGenerationOptions, ...multiFrameOptions, ...smtOptions, ...srModeOptions]
      .map(option => option.label)
    expect(labels).not.toContain('(default)')
    expect(labels).toEqual(expect.arrayContaining(['0 (off)', 'No SMT value', 'No mode', 'No override value']))
  })

  it('a later edit of a field forgets what Follow remembered', async () => {
    const replacementDesktop = desktop({
      GetProfile: vi.fn().mockResolvedValue(updatedProfile({ vkd3dHeap: true }))
    })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('VKD3D heap')).toBeTruthy())
    const heapField = screen.getByLabelText('VKD3D heap').closest('.field')
    await fireEvent.click(within(heapField).getByRole('button', { name: 'Follow All games' }))
    await fireEvent.click(screen.getByLabelText('VKD3D heap'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenLastCalledWith(1091500, [
      { field: 'proton.vkd3d_heap', operation: 'set', value: expect.any(Boolean) }
    ]))
  })

  it('pins and resets both frame generation backing fields as one action', async () => {
    const replacementDesktop = desktop()
    const view = renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByText('Frame generation')).toBeTruthy())
    const frameField = screen.getByText('Frame generation').closest('.field')
    await fireEvent.click(within(frameField).getByRole('button', { name: 'Set for this game' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'dlss.fg_enabled', operation: 'pin' },
      { field: 'dlss.fg_override', operation: 'pin' }
    ]))

    const overridden = structuredClone(fixtures.profile)
    overridden.fgEnabled = true
    overridden.fgOverride = true
    overridden.semantics = overridden.semantics.map(item =>
      item.field.startsWith('dlss.fg_') ? helper.semantic(item.field, 'override', { impact: item.impact, restore: item.restore }) : item
    )
    replacementDesktop.GetProfile.mockResolvedValue(overridden)
    replacementDesktop.PatchProfile.mockClear()
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    await waitFor(() => expect(replacementDesktop.GetProfile).toHaveBeenCalledWith(292030))
    const overriddenFrame = screen.getByText('Frame generation').closest('.field')
    await waitFor(() => expect(within(overriddenFrame).getByRole('button', { name: 'Follow All games' })).toBeTruthy())
    await fireEvent.click(within(overriddenFrame).getByRole('button', { name: 'Follow All games' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(292030, [
      { field: 'dlss.fg_enabled', operation: 'reset' },
      { field: 'dlss.fg_override', operation: 'reset' }
    ]))
  })

  it('sends a changed frame generation mode as both backing fields in one batch', async () => {
    const replacementDesktop = desktop()
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByText('Frame generation')).toBeTruthy())
    const frameField = screen.getByText('Frame generation').closest('.field')
    await fireEvent.click(frameField.querySelector('.trigger'))
    await fireEvent.click(within(frameField).getByRole('button', { name: 'Enabled' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'dlss.fg_enabled', operation: 'set', value: true },
      { field: 'dlss.fg_override', operation: 'set', value: true }
    ]))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1)
  })

  it('sends false and numeric zero as explicit sets, one batch per change', async () => {
    const replacementDesktop = desktop()
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'proton.enable_hdr', operation: 'set', value: false }
    ]))
    const clock = screen.getByLabelText('Clock offset')
    await fireEvent.input(clock, { target: { value: '0' } })
    await fireEvent.change(clock)
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'gpu.clock_offset', operation: 'set', value: 0 }
    ]))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(2)
  })

  it('saves a typed whole number from a number input and ignores anything else', async () => {
    const replacementDesktop = desktop()
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('Fan speed')).toBeTruthy())
    const fan = screen.getByLabelText('Fan speed')
    expect(fan.value).toBe('0')

    await fireEvent.input(fan, { target: { value: 'fast' } })
    await fireEvent.change(fan)
    expect(screen.getByRole('alert').textContent).toMatch(/Enter a whole number/)
    expect(fan.value).toBe('0')
    expect(replacementDesktop.PatchProfile).not.toHaveBeenCalled()

    await fireEvent.input(fan, { target: { value: '45' } })
    await fireEvent.change(fan)
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'gpu.fan_speed', operation: 'set', value: 45 }
    ]))
    expect(screen.queryByRole('alert')).toBeNull()

    const power = screen.getByLabelText('Power limit')
    await fireEvent.input(power, { target: { value: '-5' } })
    await fireEvent.change(power)
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenLastCalledWith(1091500, [
      { field: 'gpu.power_limit', operation: 'set', value: -5 }
    ]))
  })

  it('shows a stored value the list does not offer instead of a placeholder', async () => {
    const stored = updatedProfile({ powerMizer: 'prefer_future_mode', srMode: 'surprise' })
    const unknownSemantic = stored.semantics.find(item => item.field === 'gpu.power_mizer')
    unknownSemantic.valueText = 'prefer_future_mode'
    stored.semantics.find(item => item.field === 'dlss.sr_mode').valueText = 'surprise'
    const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(stored) })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    const powerField = screen.getByText('Power mode').closest('.field')
    expect(within(powerField).getByText('prefer_future_mode')).toBeTruthy()
    expect(within(powerField).queryByText('Select...')).toBeNull()
    const srField = screen.getByText('Super resolution').closest('.field')
    expect(within(srField).getByText('surprise')).toBeTruthy()
  })

  it('shows the fields that had no control: fan speed, power limit, and affinity', async () => {
    const replacementDesktop = desktop()
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('Fan speed')).toBeTruthy())
    expect(screen.getByLabelText('Power limit')).toBeTruthy()
    expect(screen.getByLabelText('Memory offset')).toBeTruthy()
  })

  it('sends optional SMT nil representation as an explicit set', async () => {
    const profile = structuredClone(fixtures.profile)
    profile.smt = 'true'
    const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(profile) })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    const smtField = screen.getByText('SMT', { selector: 'label' }).closest('.field')
    await fireEvent.click(smtField.querySelector('.trigger'))
    await fireEvent.click(within(smtField).getByRole('button', { name: 'No SMT value' }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
      { field: 'cpu.smt', operation: 'set', value: '' }
    ]))
  })

  it('offers explicit unset for the defaults root and sends it at once', async () => {
    const replacementDesktop = desktop({ GetDefaultProfile: vi.fn().mockResolvedValue(structuredClone(fixtures.defaultProfile)) })
    render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    expect(screen.queryByText('Save All games')).toBeNull()
    await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Use system default' }))
    await waitFor(() => expect(replacementDesktop.PatchDefaultProfile).toHaveBeenCalledWith([
      { field: 'proton.enable_hdr', operation: 'reset' }
    ]))
  })

  it('runs saves one after the other and lets only the newest answer replace the controls', async () => {
    const first = deferred()
    const replacementDesktop = desktop({
      PatchProfile: vi.fn().mockReturnValueOnce(first.promise).mockResolvedValueOnce(updatedProfile({ enableHdr: true, enableWayland: true }))
    })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1))
    await fireEvent.click(screen.getByLabelText('Wayland'))
    // The second write waits for the first.
    await new Promise(resolve => setTimeout(resolve, 20))
    expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1)

    first.resolve(updatedProfile({ enableHdr: false, enableWayland: false }))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(2))
    expect(replacementDesktop.PatchProfile).toHaveBeenNthCalledWith(2, 1091500, [
      { field: 'proton.enable_wayland', operation: 'set', value: true }
    ])
    await waitFor(() => expect(screen.getByText('Saved at once')).toBeTruthy())
    expect(screen.getByLabelText('HDR').checked).toBe(true)
    expect(screen.getByLabelText('Wayland').checked).toBe(true)
  })

  it('snapshots the target app before awaiting the save', async () => {
    const pending = deferred()
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockReturnValue(pending.promise) })
    const view = renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
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
    expect(screen.queryByText(/Saved: HDR/)).toBeNull()
  })

  it('does not show a stale save error after navigating away', async () => {
    const pending = deferred()
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockReturnValue(pending.promise) })
    const view = renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalled())
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    pending.reject(new Error('old target failed'))
    await waitFor(() => expect(replacementDesktop.GetProfile).toHaveBeenCalledWith(292030))
    expect(screen.queryByText(/Not saved: old target failed/)).toBeNull()
  })

  it('removes the controls of the game being left until the next profile arrives', async () => {
    const next = deferred()
    const replacementDesktop = desktop({
      GetProfile: vi.fn().mockImplementation(appId => appId === 292030 ? next.promise : Promise.resolve(structuredClone(fixtures.profile)))
    })
    const view = renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await view.rerender({
      desktop: replacementDesktop,
      game: { appId: 292030, name: 'The Witcher 3', installDir: '/games/witcher', dlls: [] },
      profileMode: 'game'
    })
    await waitFor(() => expect(replacementDesktop.GetProfile).toHaveBeenCalledWith(292030))
    expect(screen.queryByLabelText('HDR')).toBeNull()
    expect(replacementDesktop.PatchProfile).not.toHaveBeenCalled()
    next.resolve(structuredClone(fixtures.profile))
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
  })

  it('refreshes the game once, when a save creates its profile', async () => {
    const replacementDesktop = desktop({
      PatchProfile: vi.fn().mockResolvedValue(updatedProfile({ enableHdr: false })),
      GetGame: vi.fn().mockResolvedValue({ ...cyberpunk, hasProfile: true })
    })
    renderGame(replacementDesktop, { ...cyberpunk, hasProfile: false })
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(replacementDesktop.GetGame).toHaveBeenCalledWith(1091500))
    await fireEvent.click(screen.getByLabelText('Wayland'))
    await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(2))
    expect(replacementDesktop.GetGame).toHaveBeenCalledTimes(1)
  })

  describe('header and whole-page Follow', () => {
    function withClearAll(profile, count, action = 'Follow All games for all settings', body = 'This clears the settings that this game sets itself. It is saved at once.') {
      return { ...profile, clearAll: { action, count, body } }
    }

    it('names what the game sets itself, or says that every value follows All games', async () => {
      const own = withClearAll(updatedProfile(), 2)
      const view = renderGame(desktop({ GetProfile: vi.fn().mockResolvedValue(own) }))
      await waitFor(() => expect(screen.getByText('Own settings (2): Super resolution Quality · VKD3D heap Off')).toBeTruthy())
      expect(screen.getByText('DLLs')).toBeTruthy()
      expect(screen.getByText('none detected')).toBeTruthy()
      view.unmount()

      renderGame(desktop({ GetProfile: vi.fn().mockResolvedValue(withClearAll(updatedProfile({}, {}), 0)) }))
      await waitFor(() => expect(screen.getByText('Own settings: none, every value follows All games')).toBeTruthy())
    })

    it('disables Follow all settings when the page sets nothing itself', async () => {
      renderGame(desktop({ GetProfile: vi.fn().mockResolvedValue(withClearAll(updatedProfile({}, {}), 0)) }))
      await waitFor(() => expect(screen.getByRole('button', { name: 'Follow All games for all settings' })).toBeTruthy())
      expect(screen.getByRole('button', { name: 'Follow All games for all settings' }).disabled).toBe(true)
    })

    it('confirms with Cancel first and clears every own setting in one batch', async () => {
      const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(withClearAll(updatedProfile(), 2)) })
      renderGame(replacementDesktop)
      await waitFor(() => expect(screen.getByText(/^Own settings \(2\)/)).toBeTruthy())
      await fireEvent.click(screen.getByRole('button', { name: 'Follow All games for all settings' }))

      const dialog = await screen.findByRole('dialog', { name: 'Follow All games for all settings' })
      expect(within(dialog).getByText('Cyberpunk 2077')).toBeTruthy()
      expect(within(dialog).getByText(/sets itself/)).toBeTruthy()
      await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' })))

      await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
      expect(screen.queryByRole('dialog')).toBeNull()
      expect(replacementDesktop.PatchProfile).not.toHaveBeenCalled()

      await fireEvent.click(screen.getByRole('button', { name: 'Follow All games for all settings' }))
      const again = await screen.findByRole('dialog')
      await fireEvent.click(within(again).getByRole('button', { name: 'Follow All games for all settings' }))
      await waitFor(() => expect(replacementDesktop.PatchProfile).toHaveBeenCalledWith(1091500, [
        { field: 'dlss.sr_mode', operation: 'reset' },
        { field: 'proton.vkd3d_heap', operation: 'reset' }
      ]))
      expect(replacementDesktop.PatchProfile).toHaveBeenCalledTimes(1)
      await waitFor(() => expect(screen.getByText('Saved: All settings follow All games')).toBeTruthy())
    })

    it('closes the confirmation with Escape without writing', async () => {
      const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(withClearAll(updatedProfile(), 2)) })
      renderGame(replacementDesktop)
      await waitFor(() => expect(screen.getByText(/^Own settings \(2\)/)).toBeTruthy())
      await fireEvent.click(screen.getByRole('button', { name: 'Follow All games for all settings' }))
      const dialog = await screen.findByRole('dialog')
      await fireEvent.keyDown(dialog, { key: 'Escape' })
      expect(screen.queryByRole('dialog')).toBeNull()
      expect(replacementDesktop.PatchProfile).not.toHaveBeenCalled()
    })

    it('uses the system default wording on All games and counts the games that have their own value', async () => {
      const defaults = withClearAll(structuredClone(fixtures.defaultProfile), 1, 'Use system default for all settings', 'This clears the 1 setting that All games sets itself.')
      defaults.semantics = helper.fieldSemantics(helper.allOverride(), { allGames: true }).map(item => item.field === 'proton.vkd3d_heap'
        ? { ...item, ownValueSuffix: '2 games have their own value', ownValueCount: 2, ownValueAction: 'Apply to games with their own value' }
        : item)
      const replacementDesktop = desktop({ GetDefaultProfile: vi.fn().mockResolvedValue(defaults) })
      render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
      await waitFor(() => expect(screen.getByRole('button', { name: 'Use system default for all settings' })).toBeTruthy())
      expect(screen.queryByText(/^Own settings/)).toBeNull()
      const heap = screen.getByLabelText('VKD3D heap').closest('.field')
      expect(within(heap).getByText(/2 games have their own value/)).toBeTruthy()
      expect(within(heap).getByRole('button', { name: 'Apply to games with their own value' })).toBeTruthy()
      expect(screen.queryByText(/does not reach/)).toBeNull()
      expect(screen.queryByText(/Applied when the game starts/)).toBeNull()
      const hdr = screen.getByLabelText('HDR').closest('.field')
      expect(within(hdr).queryByRole('button', { name: 'Apply to games with their own value' })).toBeNull()
    })
  })

  describe('Apply to games with their own value', () => {
    const body = 'VKD3D heap: 2 games have their own value: Alpha, Beta. They will follow All games (On). It is saved at once and cannot be undone.'

    function allGamesWithTwoOwners(extra = {}) {
      const defaults = { ...structuredClone(fixtures.defaultProfile), clearAll: { action: 'Use system default for all settings', count: 1, body: '' } }
      defaults.semantics = helper.fieldSemantics(helper.allOverride(), { allGames: true }).map(item => item.field === 'proton.vkd3d_heap'
        ? { ...item, ownValueSuffix: '2 games have their own value', ownValueCount: 2, ownValueAction: 'Apply to games with their own value' }
        : item)
      return { ...defaults, ...extra }
    }

    function renderAllGames(overrides) {
      const defaults = allGamesWithTwoOwners()
      const replacementDesktop = desktop({
        GetDefaultProfile: vi.fn().mockResolvedValue(defaults),
        PreviewApplyToGamesWithOwnValue: vi.fn().mockResolvedValue({ action: 'Apply to games with their own value', count: 2, body, appIds: [11, 22] }),
        ...overrides
      })
      render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
      return replacementDesktop
    }

    async function openQuestion() {
      await waitFor(() => expect(screen.getByLabelText('VKD3D heap')).toBeTruthy())
      const heap = screen.getByLabelText('VKD3D heap').closest('.field')
      await fireEvent.click(within(heap).getByRole('button', { name: 'Apply to games with their own value' }))
      return screen.findByRole('dialog')
    }

    it('asks first with the sentence from Go, Cancel focused, and writes nothing on Cancel', async () => {
      const replacementDesktop = renderAllGames()
      const dialog = await openQuestion()
      expect(replacementDesktop.PreviewApplyToGamesWithOwnValue).toHaveBeenCalledWith(['proton.vkd3d_heap'])
      expect(within(dialog).getByText('All games')).toBeTruthy()
      expect(within(dialog).getByText(body)).toBeTruthy()
      expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' }))
      await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
      expect(screen.queryByRole('dialog')).toBeNull()
      expect(replacementDesktop.ApplyToGamesWithOwnValue).not.toHaveBeenCalled()
    })

    it('applies on confirm, shows the refreshed page and says what happened', async () => {
      const refreshed = allGamesWithTwoOwners({ semantics: helper.fieldSemantics(helper.allOverride(), { allGames: true }) })
      const replacementDesktop = renderAllGames({
        ApplyToGamesWithOwnValue: vi.fn().mockResolvedValue({ message: 'VKD3D heap: 2 games follow All games', failed: [], profile: refreshed })
      })
      const dialog = await openQuestion()
      await fireEvent.click(within(dialog).getByRole('button', { name: 'Apply to games with their own value' }))
      await waitFor(() => expect(replacementDesktop.ApplyToGamesWithOwnValue).toHaveBeenCalledWith(['proton.vkd3d_heap'], [11, 22]))
      expect(replacementDesktop.ApplyToGamesWithOwnValue).toHaveBeenCalledTimes(1)
      await waitFor(() => expect(screen.getByText('Saved: VKD3D heap: 2 games follow All games')).toBeTruthy())
      expect(screen.queryByRole('dialog')).toBeNull()
      const heap = screen.getByLabelText('VKD3D heap').closest('.field')
      expect(within(heap).queryByRole('button', { name: 'Apply to games with their own value' })).toBeNull()
      expect(within(heap).queryByText(/have their own value/)).toBeNull()
    })

    it('names the game that could not be written and keeps the page', async () => {
      const failure = 'Not saved for Beta (disk full). VKD3D heap: 1 game follows All games'
      const replacementDesktop = renderAllGames({
        ApplyToGamesWithOwnValue: vi.fn().mockResolvedValue({ message: failure, failure: 'Beta (disk full)', failed: ['Beta'], profile: allGamesWithTwoOwners() })
      })
      const dialog = await openQuestion()
      await fireEvent.click(within(dialog).getByRole('button', { name: 'Apply to games with their own value' }))
      await waitFor(() => expect(screen.getAllByText(failure).length).toBeGreaterThan(0))
      // The header says it once, not "Not saved: Not saved for Beta".
      expect(screen.getAllByText('Not saved: Beta (disk full)').length).toBeGreaterThan(0)
      expect(screen.queryByText(/Not saved: Not saved/)).toBeNull()
      expect(screen.queryByText(/^Saved: VKD3D heap/)).toBeNull()
      expect(replacementDesktop.ApplyToGamesWithOwnValue).toHaveBeenCalledTimes(1)
    })

    it('shows a rejected apply as Not saved', async () => {
      renderAllGames({ ApplyToGamesWithOwnValue: vi.fn().mockRejectedValue(new Error('read-only')) })
      const dialog = await openQuestion()
      await fireEvent.click(within(dialog).getByRole('button', { name: 'Apply to games with their own value' }))
      await waitFor(() => expect(screen.getAllByText('Not saved: read-only').length).toBeGreaterThan(0))
    })

    it('passes both fields for the frame generation control', async () => {
      const defaults = allGamesWithTwoOwners()
      defaults.semantics = defaults.semantics.map(item => item.field === 'dlss.fg_override'
        ? { ...item, ownValueSuffix: '1 game has its own value', ownValueCount: 1, ownValueAction: 'Apply to games with their own value' }
        : item)
      const replacementDesktop = desktop({
        GetDefaultProfile: vi.fn().mockResolvedValue(defaults),
        PreviewApplyToGamesWithOwnValue: vi.fn().mockResolvedValue({ action: 'Apply to games with their own value', count: 1, body: 'Frame generation: 1 game has their own value: Alpha.' })
      })
      render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
      await waitFor(() => expect(screen.getByText('Frame generation')).toBeTruthy())
      const field = screen.getByText('Frame generation').closest('.field')
      await fireEvent.click(within(field).getByRole('button', { name: 'Apply to games with their own value' }))
      await waitFor(() => expect(replacementDesktop.PreviewApplyToGamesWithOwnValue).toHaveBeenCalledWith(['dlss.fg_enabled', 'dlss.fg_override']))
    })

    it('is not offered on a game page', async () => {
      const gameProfile = { ...structuredClone(fixtures.profile), semantics: helper.fieldSemantics({ 'proton.vkd3d_heap': 'override' }).map(item => ({ ...item, ownValueCount: 2, ownValueAction: 'Apply to games with their own value' })) }
      const replacementDesktop = desktop({ GetProfile: vi.fn().mockResolvedValue(gameProfile) })
      renderGame(replacementDesktop)
      await waitFor(() => expect(screen.getByLabelText('VKD3D heap')).toBeTruthy())
      expect(screen.queryByRole('button', { name: 'Apply to games with their own value' })).toBeNull()
    })
  })

  describe('All games row buttons', () => {
    function defaults(sources, changes = {}) {
      return { ...structuredClone(fixtures.defaultProfile), ...changes, semantics: helper.fieldSemantics(sources, { allGames: true }), clearAll: { action: 'Use system default for all settings', count: Object.keys(sources).length, body: '' } }
    }

    it('offers Set here, never a no-op Use system default, on a value that is not set there', async () => {
      const replacementDesktop = desktop({ GetDefaultProfile: vi.fn().mockResolvedValue(defaults({})) })
      render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
      await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
      const field = screen.getByLabelText('HDR').closest('.field')
      expect(within(field).queryByRole('button', { name: 'Use system default' })).toBeNull()
      await fireEvent.click(within(field).getByRole('button', { name: 'Set here' }))
      await waitFor(() => expect(replacementDesktop.PatchDefaultProfile).toHaveBeenCalledWith([
        { field: 'proton.enable_hdr', operation: 'set', value: true }
      ]))
      expect(replacementDesktop.PatchDefaultProfile).toHaveBeenCalledTimes(1)
    })

    it('puts the value back with Set here after Use system default', async () => {
      const own = defaults({ 'proton.enable_hdr': 'override' }, { enableHdr: true })
      const cleared = defaults({}, { enableHdr: false })
      const replacementDesktop = desktop({
        GetDefaultProfile: vi.fn().mockResolvedValue(own),
        PatchDefaultProfile: vi.fn().mockResolvedValueOnce(cleared).mockResolvedValueOnce(own)
      })
      render(GameDetail, { props: { desktop: replacementDesktop, game: null, profileMode: 'default' } })
      await waitFor(() => expect(screen.getByLabelText('HDR').checked).toBe(true))
      await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Use system default' }))
      await waitFor(() => expect(screen.getByLabelText('HDR').checked).toBe(false))
      await fireEvent.click(within(screen.getByLabelText('HDR').closest('.field')).getByRole('button', { name: 'Set here' }))
      await waitFor(() => expect(replacementDesktop.PatchDefaultProfile).toHaveBeenLastCalledWith([
        { field: 'proton.enable_hdr', operation: 'set', value: true }
      ]))
      await waitFor(() => expect(screen.getByLabelText('HDR').checked).toBe(true))
    })
  })

  it('shows the stored number again at once when a number save fails, even while the field has focus', async () => {
    const replacementDesktop = desktop({ PatchProfile: vi.fn().mockRejectedValueOnce(new Error('read-only')) })
    renderGame(replacementDesktop)
    await waitFor(() => expect(screen.getByLabelText('Fan speed')).toBeTruthy())
    const fan = screen.getByLabelText('Fan speed')
    fan.focus()
    await fireEvent.input(fan, { target: { value: '45' } })
    await fireEvent.change(fan)
    await waitFor(() => expect(screen.getAllByText('Not saved: read-only').length).toBeGreaterThan(0))
    expect(document.activeElement).toBe(fan)
    expect(fan.value).toBe('0')
  })
})
