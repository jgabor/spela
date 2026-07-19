import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'

import DLLCatalogPane from './DLLCatalogPane.svelte'
import GameDLLPane from './GameDLLPane.svelte'
import GameOverview from './GameOverview.svelte'
import Monitor from './Monitor.svelte'
import SettingsPane from './SettingsPane.svelte'

afterEach(() => cleanup())

describe('monitor supported sections', () => {
  it('renders GPU, CPU, alerts, and unavailable snapshots', async () => {
    const desktop = {
      GetGPUInfo: vi.fn().mockResolvedValue({
        name: 'RTX 5090', temperature: 72, utilization: 96,
        powerDraw: 420.4, powerLimit: 500, memoryUsed: 12000,
        memoryTotal: 32000, graphicsClock: 2900, memoryClock: 14000
      }),
      GetCPUInfo: vi.fn().mockResolvedValue({
        model: 'Ryzen', cores: 16, averageFrequency: 5100,
        governor: 'performance', smtEnabled: false
      })
    }
    const view = render(Monitor, { props: { desktop, section: 0 } })
    await waitFor(() => expect(screen.getByText('RTX 5090')).toBeTruthy())
    expect(screen.getByText('420 / 500 W')).toBeTruthy()

    await view.rerender({ desktop, section: 1 })
    expect(screen.getByText('Ryzen')).toBeTruthy()
    expect(screen.getByText('disabled')).toBeTruthy()

    await view.rerender({ desktop, section: 2 })
    expect(screen.getByText(/No active alerts/)).toBeTruthy()

    desktop.GetGPUInfo.mockResolvedValue(null)
    desktop.GetCPUInfo.mockResolvedValue(null)
    view.unmount()
    render(Monitor, { props: { desktop, section: 0 } })
    await waitFor(() => expect(screen.getByText('GPU metrics unavailable')).toBeTruthy())
  })
})

describe('settings supported controls', () => {
  it('projects toggle, select, text, status, and empty section behavior', async () => {
    const onOptionChange = vi.fn()
    const section = {
      title: 'All controls',
      options: [
        { key: 'hints', label: 'Show hints', description: 'Hints', type: 'toggle' },
        { key: 'level', label: 'Log level', description: 'Logging', type: 'select', choices: ['info', 'debug'] },
        { key: 'path', label: 'Steam path', description: 'Steam', type: 'text' }
      ]
    }
    const view = render(SettingsPane, {
      props: {
        section,
        optionsState: { hints: false, level: 'info', path: '' },
        configMessage: 'Saved',
        configMessageType: 'success',
        onOptionChange
      }
    })
    expect(screen.getByText('Saved').dataset.type).toBe('success')
    await fireEvent.click(screen.getByRole('button', { name: 'Show hints' }))
    await fireEvent.change(screen.getByRole('combobox'), { target: { value: 'debug' } })
    await fireEvent.change(screen.getByLabelText('Steam path'), { target: { value: '/steam' } })
    expect(onOptionChange.mock.calls).toEqual([
      ['hints', true], ['level', 'debug'], ['path', '/steam']
    ])

    await view.rerender({ section: null, optionsState: {}, onOptionChange })
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeTruthy()
  })
})

describe('DLL catalog and overview supported projections', () => {
  it('renders inventory, deployment rows, game details, and empty details', async () => {
    const desktop = {
      GetGames: vi.fn().mockResolvedValue([
        { name: 'Cyberpunk 2077', dlls: [{ type: 'dlss' }, { type: 'dlssg' }] },
        { name: 'Hades', dlls: [] }
      ])
    }
    const catalog = render(DLLCatalogPane, { props: { desktop, section: 0 } })
    expect(screen.getByRole('heading', { name: 'DLL Library' })).toBeTruthy()
    await catalog.rerender({ desktop, section: 1 })
    await waitFor(() => expect(screen.getByText('Cyberpunk 2077')).toBeTruthy())
    expect(screen.getByText('dlss, dlssg')).toBeTruthy()
    expect(screen.getByText('—')).toBeTruthy()
    catalog.unmount()

    const overview = render(GameOverview, { props: { game: null } })
    expect(screen.getByText(/Select a game/)).toBeTruthy()
    await overview.rerender({
      game: {
        appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cp',
        prefixPath: '/prefix', dlls: [{ type: 'dlss', version: '3.8.10' }, { type: 'xess', version: '' }]
      }
    })
    expect(screen.getByText('/prefix')).toBeTruthy()
    expect(screen.getByText('3.8.10')).toBeTruthy()
    expect(screen.getByText('?')).toBeTruthy()
    expect(screen.getByText(/spela %command%/)).toBeTruthy()
  })

  it('keeps DLL catalog load failures visible', async () => {
    render(DLLCatalogPane, {
      props: { desktop: { GetGames: vi.fn().mockRejectedValue(new Error('database unavailable')) }, section: 1 }
    })
    await waitFor(() => expect(screen.getByText(/database unavailable/)).toBeTruthy())
  })
})

function dllDesktop(overrides = {}) {
  let progressHandler
  const unsubscribe = vi.fn()
  return {
    desktop: {
      SubscribeDLLProgress: vi.fn(handler => { progressHandler = handler; return unsubscribe }),
      CheckDLLUpdates: vi.fn().mockResolvedValue([
        { dllType: 'dlss', currentVersion: '3.7.0', latestVersion: '3.8.10', hasUpdate: true }
      ]),
      HasDLLBackup: vi.fn().mockResolvedValue(true),
      GetGame: vi.fn().mockResolvedValue({
        appId: 1091500, name: 'Cyberpunk 2077',
        dlls: [{ dllType: 'dlss', version: '3.8.10' }]
      }),
      ListDLLInstallTypes: vi.fn().mockResolvedValue(['dlss', 'xess']),
      ListDLLVersions: vi.fn().mockResolvedValue(['3.8.10', '3.7.0']),
      InstallDLL: vi.fn().mockResolvedValue(undefined),
      UpdateDLLs: vi.fn().mockResolvedValue({ updated: 1, unchanged: 0, failed: 0, failures: [] }),
      RestoreDLLs: vi.fn().mockResolvedValue(undefined),
      ...overrides
    },
    emitProgress(stage) { progressHandler?.(stage) },
    unsubscribe
  }
}

describe('game DLL mutation adapter flow', () => {
  const game = {
    appId: 1091500, name: 'Cyberpunk 2077',
    dlls: [{ dllType: 'dlss', version: '3.7.0' }]
  }

  it('updates, restores, and installs through the supported desktop adapter', async () => {
    const adapter = dllDesktop()
    const view = render(GameDLLPane, { props: { game, desktop: adapter.desktop } })
    await waitFor(() => expect(screen.getByText('Update all DLLs')).toBeTruthy())
    expect(screen.getByText('Backup available')).toBeTruthy()

    await fireEvent.click(screen.getByText('Update all DLLs'))
    await waitFor(() => expect(adapter.desktop.UpdateDLLs).toHaveBeenCalledWith(1091500))
    await waitFor(() => expect(adapter.desktop.GetGame).toHaveBeenCalledWith(1091500))

    await fireEvent.click(screen.getByText('Restore original DLLs'))
    await waitFor(() => expect(adapter.desktop.RestoreDLLs).toHaveBeenCalledWith(1091500))

    await fireEvent.click(screen.getByText('Install DLL'))
    await waitFor(() => expect(screen.getByRole('dialog', { name: 'Install DLL' })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: 'DLSS', exact: true }))
    await waitFor(() => expect(screen.getByText('3.8.10 (latest)')).toBeTruthy())
    adapter.emitProgress('Downloading')
    await fireEvent.click(screen.getByText('3.8.10 (latest)'))
    await waitFor(() => expect(adapter.desktop.InstallDLL).toHaveBeenCalledWith(1091500, 'dlss', '3.8.10'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Install DLL' })).toBeNull())

    view.unmount()
    expect(adapter.unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('shows supported install errors and permits closing or backing up the wizard', async () => {
    const adapter = dllDesktop({
      ListDLLVersions: vi.fn().mockResolvedValue([]),
      UpdateDLLs: vi.fn().mockRejectedValue('update failed')
    })
    render(GameDLLPane, { props: { game, desktop: adapter.desktop } })
    await waitFor(() => expect(screen.getByText('Update all DLLs')).toBeTruthy())
    await fireEvent.click(screen.getByText('Update all DLLs'))
    await waitFor(() => expect(adapter.desktop.UpdateDLLs).toHaveBeenCalled())

    await fireEvent.click(screen.getByText('Install DLL'))
    await fireEvent.click(await screen.findByText('XeSS'))
    await waitFor(() => expect(screen.getByText('No versions available for XeSS.')).toBeTruthy())
    await fireEvent.click(screen.getByText('Back'))
    expect(screen.getByText('Select DLL type')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Close install' }))
    expect(screen.queryByRole('dialog', { name: 'Install DLL' })).toBeNull()
  })
})
