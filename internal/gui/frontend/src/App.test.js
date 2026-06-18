import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const fixtures = vi.hoisted(() => ({
  games: [
    { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
    { appId: 1145360, name: 'Hades', installDir: '/games/hades', dlls: [] }
  ],
  profile: {
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
    inheritedFromDefault: false,
    semantics: []
  },
  settingsCatalog: [
    { id: 0, title: 'Display', options: [{ key: 'showHints', label: 'Show hints', description: '', type: 'toggle' }] },
    { id: 1, title: 'Startup', options: [] },
    { id: 2, title: 'Paths', options: [{ key: 'steamPath', label: 'Steam path', description: '', type: 'text' }] },
    { id: 3, title: 'DLL policy', options: [] },
    { id: 4, title: 'Logging', options: [{ key: 'logLevel', label: 'Log level', description: '', type: 'select', choices: ['info'] }] }
  ],
  defaultNav: {
    destination: 0,
    scopeGlobal: true,
    gameName: '',
    aspect: 1,
    subsystem: 0,
    dllSection: 0,
    monitorSection: 0,
    settingsSection: 0,
    breadcrumb: ['Library', 'All games', 'Profile', 'Proton']
  },
  config: {
    theme: 'dark',
    showHints: true,
    compactMode: false,
    logLevel: 'info',
    steamPath: '',
    dllCachePath: '',
    backupPath: '',
    confirmDestructive: true,
    rescanOnStartup: false,
    autoUpdateDLLs: false,
    checkUpdates: false,
    autoRefreshManifest: true,
    manifestRefreshHours: 24,
    preferredDLLSource: 'techpowerup'
  }
}))

vi.mock('../wailsjs/go/gui/App', () => ({
  CheckDLLUpdates: vi.fn().mockResolvedValue([]),
  DefaultNavState: vi.fn().mockImplementation(async () => structuredClone(fixtures.defaultNav)),
  GetNavContract: vi.fn().mockResolvedValue({}),
  NavDestinationFromHotkey: vi.fn().mockReturnValue([0, false]),
  NavSelectAspect: vi.fn().mockImplementation(async (state, aspect) => ({ ...state, aspect })),
  NavSelectDLLSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, dllSection: section })),
  NavSelectDestination: vi.fn().mockImplementation(async (state, destination) => ({ ...state, destination })),
  NavSelectMonitorSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, monitorSection: section })),
  NavSelectScope: vi.fn().mockImplementation(async (state, scopeGlobal, gameName = '') => ({
    ...state,
    scopeGlobal,
    gameName,
    aspect: scopeGlobal ? 1 : state.aspect
  })),
  NavSelectSettingsSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, settingsSection: section })),
  NavSelectSubsystem: vi.fn().mockImplementation(async (state, subsystem) => ({ ...state, subsystem })),
  GetCPUInfo: vi.fn().mockResolvedValue({ utilizationPercent: 12, averageFrequency: 4200, memoryUsedMegabytes: 8192, memoryTotalMegabytes: 32768 }),
  GetConfig: vi.fn().mockResolvedValue(fixtures.config),
  GetDefaultProfile: vi.fn().mockResolvedValue(fixtures.profile),
  GetGPUInfo: vi.fn().mockResolvedValue({ temperature: 62, utilization: 44, powerDraw: 180, memoryUsed: 4096, memoryTotal: 12288 }),
  GetGame: vi.fn().mockResolvedValue(fixtures.games[0]),
  GetGames: vi.fn().mockResolvedValue(fixtures.games),
  GetLogo: vi.fn().mockResolvedValue(''),
  GetProfile: vi.fn().mockResolvedValue(fixtures.profile),
  GetSettingsCatalog: vi.fn().mockResolvedValue(fixtures.settingsCatalog),
  GetVersion: vi.fn().mockResolvedValue('0.6.0'),
  HasDLLBackup: vi.fn().mockResolvedValue(false),
  InstallDLL: vi.fn().mockResolvedValue(undefined),
  LaunchGame: vi.fn().mockResolvedValue(undefined),
  ListDLLInstallTypes: vi.fn().mockResolvedValue([]),
  ListDLLVersions: vi.fn().mockResolvedValue([]),
  RestoreDLLs: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  SaveDefaultProfile: vi.fn().mockResolvedValue(undefined),
  SaveProfile: vi.fn().mockResolvedValue(undefined),
  ScanGames: vi.fn().mockResolvedValue(undefined),
  UpdateDLLs: vi.fn().mockResolvedValue(undefined),
  VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue('')
}))

vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn().mockReturnValue(() => {}),
  Quit: vi.fn()
}))

import App from './App.svelte'
import { Quit } from '../wailsjs/runtime/runtime'

function makeDesktop(overrides = {}) {
  return {
    CheckDLLUpdates: vi.fn().mockResolvedValue([]),
  DefaultNavState: vi.fn().mockImplementation(async () => structuredClone(fixtures.defaultNav)),
  GetNavContract: vi.fn().mockResolvedValue({}),
  NavDestinationFromHotkey: vi.fn().mockReturnValue([0, false]),
  NavSelectAspect: vi.fn().mockImplementation(async (state, aspect) => ({ ...state, aspect })),
  NavSelectDLLSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, dllSection: section })),
  NavSelectDestination: vi.fn().mockImplementation(async (state, destination) => ({ ...state, destination })),
  NavSelectMonitorSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, monitorSection: section })),
  NavSelectScope: vi.fn().mockImplementation(async (state, scopeGlobal, gameName = '') => ({
    ...state,
    scopeGlobal,
    gameName,
    aspect: scopeGlobal ? 1 : state.aspect
  })),
  NavSelectSettingsSection: vi.fn().mockImplementation(async (state, section) => ({ ...state, settingsSection: section })),
  NavSelectSubsystem: vi.fn().mockImplementation(async (state, subsystem) => ({ ...state, subsystem })),
    GetCPUInfo: vi.fn().mockResolvedValue({ utilizationPercent: 12, averageFrequency: 4200, memoryUsedMegabytes: 8192, memoryTotalMegabytes: 32768 }),
    GetConfig: vi.fn().mockResolvedValue(fixtures.config),
    GetDefaultProfile: vi.fn().mockResolvedValue(fixtures.profile),
    GetGPUInfo: vi.fn().mockResolvedValue({ temperature: 62, utilization: 44, powerDraw: 180, memoryUsed: 4096, memoryTotal: 12288 }),
    GetGame: vi.fn(appId => Promise.resolve(fixtures.games.find(game => game.appId === appId))),
    GetGames: vi.fn().mockResolvedValue(fixtures.games),
    GetLogo: vi.fn().mockResolvedValue(''),
    GetProfile: vi.fn().mockResolvedValue(fixtures.profile),
    GetSettingsCatalog: vi.fn().mockResolvedValue(fixtures.settingsCatalog),
    GetVersion: vi.fn().mockResolvedValue('0.6.0'),
    HasDLLBackup: vi.fn().mockResolvedValue(false),
    InstallDLL: vi.fn().mockResolvedValue(undefined),
    LaunchGame: vi.fn().mockResolvedValue(undefined),
    ListDLLInstallTypes: vi.fn().mockResolvedValue([]),
    ListDLLVersions: vi.fn().mockResolvedValue([]),
    RestoreDLLs: vi.fn().mockResolvedValue(undefined),
    SaveConfig: vi.fn().mockResolvedValue(undefined),
    SaveDefaultProfile: vi.fn().mockResolvedValue(undefined),
    SaveProfile: vi.fn().mockResolvedValue(undefined),
    ScanGames: vi.fn().mockResolvedValue(undefined),
    SubscribeDLLProgress: vi.fn().mockReturnValue(() => {}),
    UpdateDLLs: vi.fn().mockResolvedValue(undefined),
    VKD3DHeapCompatibilityNotice: vi.fn().mockResolvedValue(''),
    Quit: vi.fn(),
    ...overrides
  }
}

describe('App keyboard behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('navigates library scope, settings, and help through the shell', async () => {
    const desktop = makeDesktop()
    render(App, { props: { desktop } })

    await waitFor(() => expect(screen.getByRole('button', { name: /Cyberpunk 2077/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /Cyberpunk 2077/ }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Cyberpunk 2077' })).toBeTruthy())
    expect(desktop.GetProfile).toHaveBeenCalledWith(1091500)

    await fireEvent.click(screen.getByRole('button', { name: /Hades/ }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Hades' })).toBeTruthy())
    expect(desktop.GetProfile).toHaveBeenCalledWith(1145360)

    await fireEvent.click(screen.getByRole('button', { name: /All games \(default\)/ }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'All games (default)' })).toBeTruthy())
    expect(screen.getByRole('button', { name: 'Save default profile' })).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Display' })).toBeTruthy())
    const showHintsRow = screen.getByText('Show hints').closest('.option-row')
    await fireEvent.click(within(showHintsRow).getByRole('button'))
    await waitFor(() => expect(desktop.SaveConfig).toHaveBeenCalledWith(expect.objectContaining({ showHints: false })))
    expect(screen.getByText('Options saved')).toBeTruthy()

    await fireEvent.keyDown(window, { key: '?' })
    const help = screen.getByRole('dialog', { name: 'Help' })
    expect(help).toBeTruthy()
    await fireEvent.click(within(help).getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog', { name: 'Help' })).toBeNull()
  })

  it('does not run global shortcuts while typing in editable settings controls', async () => {
    const desktop = makeDesktop()
    render(App, { props: { desktop } })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Settings' })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Display' })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: 'Paths' }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Paths' })).toBeTruthy())

    const steamPath = screen.getByLabelText('Steam path')
    await fireEvent.input(steamPath, { target: { value: '/mnt/steam' } })
    await fireEvent.keyDown(steamPath, { key: 'q' })
    await fireEvent.keyDown(steamPath, { key: '?' })
    const tabDefaultAllowed = await fireEvent.keyDown(steamPath, { key: 'Tab' })

    expect(desktop.Quit).not.toHaveBeenCalled()
    expect(Quit).not.toHaveBeenCalled()
    expect(tabDefaultAllowed).toBe(true)
    expect(screen.queryByRole('dialog', { name: 'Help' })).toBeNull()
  })

  it('keeps pane switching, help, and quit global shortcuts active outside editable controls', async () => {
    const desktop = makeDesktop()
    render(App, { props: { desktop } })

    await waitFor(() => expect(screen.getByRole('button', { name: /Cyberpunk 2077/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /Cyberpunk 2077/ }))
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Cyberpunk 2077' })).toBeTruthy())

    await fireEvent.keyDown(window, { key: 'Tab' })
    await waitFor(() => expect(document.activeElement).toBe(screen.getByPlaceholderText('Search games...')))

    await fireEvent.keyDown(window, { key: '4' })
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Display' })).toBeTruthy())

    await fireEvent.keyDown(window, { key: '?' })
    expect(screen.getByRole('dialog', { name: 'Help' })).toBeTruthy()

    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Help' })).toBeNull()

    await fireEvent.keyDown(window, { key: 'q' })
    expect(desktop.Quit).toHaveBeenCalledTimes(1)
  })
})
