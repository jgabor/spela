import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'

import GameDetail from './GameDetail.svelte'
import GameList from './GameList.svelte'
import GameProfilePane from './GameProfilePane.svelte'
import { emptyProfile } from './profileFieldOptions.js'
import { allOverride, fieldSemantics, semantic } from './testSemantics.js'

const withWords = (profile, sources, options) => ({ ...profile, semantics: fieldSemantics(sources, options) })

afterEach(() => cleanup())

const games = [
  { appId: 1, name: 'Alpha', installDir: '/alpha', dlls: [{ dllType: 'dlss' }], hasProfile: true },
  { appId: 2, name: 'Beta', installDir: '/beta', dlls: [], hasProfile: false },
  { appId: 3, name: 'Gamma', installDir: '/gamma', dlls: [{ dllType: 'xess' }], hasProfile: false }
]

describe('game list supported interaction flows', () => {
  it('rescans, filters, selects, and runs mixed batch updates', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const desktop = {
      GetGames: vi.fn().mockResolvedValue(games),
      ScanGames: vi.fn().mockResolvedValue(undefined),
      UpdateDLLs: vi.fn(appId => appId === 3
        ? Promise.reject(new Error('offline'))
        : Promise.resolve({ updated: 0, unchanged: 1, failed: 0 }))
    }
    const view = render(GameList, { props: { desktop } })
    await waitFor(() => expect(screen.getByText('Alpha')).toBeTruthy())

    await fireEvent.click(screen.getByText('Rescan'))
    await waitFor(() => expect(desktop.ScanGames).toHaveBeenCalled())
    const search = screen.getByPlaceholderText('Search games...')
    await fireEvent.input(search, { target: { value: 'missing' } })
    expect(screen.getByText('No games matching filters')).toBeTruthy()
    await fireEvent.click(screen.getByText('Clear filters'))
    expect(screen.getByText('3 games')).toBeTruthy()

    await fireEvent.click(screen.getByText('Select'))
    await fireEvent.click(screen.getByText('Select all'))
    expect(screen.getByText('3 selected')).toBeTruthy()
    await fireEvent.click(screen.getByText('Select none'))
    expect(screen.getByText('0 selected')).toBeTruthy()
    await fireEvent.click(screen.getByText('Select all'))
    await fireEvent.click(screen.getByText('Update all DLLs'))
    await waitFor(() => expect(desktop.UpdateDLLs).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.getByText('Updated 0 games, 1 already current, 1 failed, 1 skipped')).toBeTruthy())

    await fireEvent.click(screen.getByText('Select'))
    const betaCheckbox = screen.getByText('Beta').closest('label').querySelector('input')
    await fireEvent.click(betaCheckbox)
    await fireEvent.click(screen.getByText('Update all DLLs'))
    expect(screen.getByText('No selected games have DLLs to update')).toBeTruthy()
    await fireEvent.click(betaCheckbox)
    await fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByText('0 selected')).toBeNull()

    await view.component.focusSearch()
    expect(document.activeElement).toBe(search)
    await view.component.refreshGames()
    expect(desktop.GetGames.mock.calls.length).toBeGreaterThan(1)
    consoleError.mockRestore()
  })

  it('retains an empty-state contract when discovery fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const desktop = {
      GetGames: vi.fn().mockRejectedValue(new Error('database unavailable')),
      ScanGames: vi.fn().mockRejectedValue(new Error('scan unavailable'))
    }
    render(GameList, { props: { desktop } })
    await waitFor(() => expect(screen.getByText(/No games found/)).toBeTruthy())
    await fireEvent.click(screen.getByText('Rescan'))
    await waitFor(() => expect(consoleError).toHaveBeenCalled())
    consoleError.mockRestore()
  })
})

describe('profile editor supported projections', () => {
  it('uses distinct descriptor-parity options for SR and RR presets', async () => {
    render(GameProfilePane, {
      props: { profile: withWords(emptyProfile(), allOverride(), { allGames: true }), desktop: {}, profileMode: 'default', profileSubsystem: 1 }
    })

    const srField = screen.getByText('Resolution preset').closest('.field')
    await fireEvent.click(srField.querySelector('.trigger'))
    expect([...srField.querySelectorAll('.option')].map(option => option.textContent.trim())).toEqual([
      'No preset', 'Driver default', 'Auto (mode-linked)', 'A', 'B', 'C', 'D', 'E', 'F', 'J', 'K', 'L', 'M'
    ])

    const rrField = screen.getByText('Ray preset').closest('.field')
    await fireEvent.click(rrField.querySelector('.trigger'))
    expect([...rrField.querySelectorAll('.option')].map(option => option.textContent.trim())).toEqual([
      'No preset', 'Driver default', 'A', 'B', 'C', 'D', 'E', 'F', 'J', 'K', 'L', 'M'
    ])
  })

  it('projects semantics, frame generation, and compatibility notices', async () => {
    const profile = {
      ...withWords(emptyProfile(), { 'proton.vkd3d_heap': 'override' }),
      inheritedFromDefault: true,
      vkd3dHeap: true
    }
    const desktop = { VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue('⚠ Upgrade driver') }
    const view = render(GameProfilePane, {
      props: { profile, game: games[0], desktop, profileMode: 'game', profileSubsystem: null }
    })
    await waitFor(() => expect(screen.getByText('⚠ Upgrade driver')).toBeTruthy())
    expect(screen.getAllByText(/this game · All games: Off/).length).toBeGreaterThan(0)
    expect(screen.getByText('This game has no profile of its own; every value follows All games.')).toBeTruthy()

    const frameField = screen.getByText('Frame generation').closest('.field')
    await fireEvent.click(frameField.querySelector('button'))
    await waitFor(() => expect(frameField.querySelectorAll('button').length).toBeGreaterThan(1))
    await fireEvent.click([...frameField.querySelectorAll('button')].find(button => button.textContent.trim() === 'Enabled'))
    expect(profile.fgOverride).toBe(true)
    expect(profile.fgEnabled).toBe(true)
    await fireEvent.click(frameField.querySelector('button'))
    await waitFor(() => expect(frameField.querySelectorAll('button').length).toBeGreaterThan(1))
    await fireEvent.click([...frameField.querySelectorAll('button')].find(button => button.textContent.trim() === 'No override value'))
    expect(profile.fgOverride).toBe(false)

    desktop.VKD3DHeapCompatibilityNotice.mockRejectedValue(new Error('probe failed'))
    await view.rerender({ profile: { ...profile, vkd3dHeap: false }, game: games[0], desktop, profileMode: 'game', profileSubsystem: 0 })
    expect(screen.getByRole('heading', { name: 'Proton settings' })).toBeTruthy()
    expect(screen.queryByText('⚠ Upgrade driver')).toBeNull()
    await view.rerender({ profile: withWords(profile, allOverride(), { allGames: true }), game: null, desktop, profileMode: 'default', profileSubsystem: 0 })
    expect(screen.getAllByText(/^set here$/).length).toBeGreaterThan(0)
  })
})

describe('profile pane words', () => {
  it('prints labels, descriptions, sources, and actions from the projection, not from its own text', () => {
    const profile = emptyProfile()
    profile.semantics = [
      { ...semantic('dlss.sr_mode', 'override'), label: 'Zeta label', description: 'Zeta description.', sourceText: 'zeta source', action: 'Zeta action' }
    ]
    render(GameProfilePane, { props: { profile, desktop: {}, profileMode: 'game', profileSubsystem: 1 } })
    expect(screen.getByText('Zeta label')).toBeTruthy()
    expect(screen.getByText('Zeta description.')).toBeTruthy()
    expect(screen.getByText(/zeta source/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Zeta action' })).toBeTruthy()
    for (const retired of ['Quality mode', 'Pin', 'Unset', '(default)', 'Override for this game']) {
      expect(screen.queryByText(retired)).toBeNull()
    }
  })
})

describe('game detail supported persistence flows', () => {
  it('saves default and game profiles as they change and reports adapter failures', async () => {
    const profile = withWords(emptyProfile())
    const desktop = {
      GetDefaultProfile: vi.fn().mockImplementation(async () => structuredClone(profile)),
      GetProfile: vi.fn().mockImplementation(async () => structuredClone(profile)),
      PatchDefaultProfile: vi.fn().mockResolvedValue(undefined),
      PatchProfile: vi.fn().mockResolvedValue(undefined),
      GetGame: vi.fn().mockResolvedValue({ ...games[0], prefixPath: '/prefix' }),
      VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue('')
    }
    const defaults = render(GameDetail, { props: { game: null, profileMode: 'default', desktop } })
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    expect(screen.queryByText('Save All games')).toBeNull()
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(desktop.PatchDefaultProfile).toHaveBeenCalledWith([{
      field: 'proton.enable_hdr', operation: 'set', value: true
    }]))
    await waitFor(() => expect(screen.getByText(/^Saved: /)).toBeTruthy())
    defaults.unmount()

    const detail = render(GameDetail, { props: { game: games[0], profileMode: 'game', desktop } })
    await waitFor(() => expect(screen.getByLabelText('HDR')).toBeTruthy())
    expect(screen.queryByText('Save profile')).toBeNull()
    await detail.component.focusPrimary()
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(desktop.PatchProfile).toHaveBeenCalledWith(1, [{
      field: 'proton.enable_hdr', operation: 'set', value: true
    }]))
    await waitFor(() => expect(screen.getByText(/^Saved: /)).toBeTruthy())

    desktop.PatchProfile.mockRejectedValue('read-only')
    await fireEvent.click(screen.getByLabelText('HDR'))
    await waitFor(() => expect(screen.getAllByText('Not saved: read-only').length).toBeGreaterThan(0))
    await fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Dismiss')).toBeNull()
  })
})
