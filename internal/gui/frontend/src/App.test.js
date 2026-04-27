import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const fixtures = vi.hoisted(() => ({
  game: { appId: 1091500, name: 'Cyberpunk 2077', installDir: '/games/cyberpunk', dlls: [] },
  profile: {
    srMode: '',
    srPreset: '',
    srOverride: false,
    fgEnabled: false,
    fgOverride: false,
    multiFrame: 0,
    indicator: false,
    shaderCache: false,
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
    backupOnLaunch: false,
    inheritedFromDefault: false,
    semantics: []
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
  GetCPUInfo: vi.fn().mockResolvedValue({ utilizationPercent: 12, averageFrequency: 4200, memoryUsedMegabytes: 8192, memoryTotalMegabytes: 32768 }),
  GetConfig: vi.fn().mockResolvedValue(fixtures.config),
  GetDefaultProfile: vi.fn().mockResolvedValue(fixtures.profile),
  GetGPUInfo: vi.fn().mockResolvedValue({ temperature: 62, utilization: 44, powerDraw: 180, memoryUsed: 4096, memoryTotal: 12288 }),
  GetGame: vi.fn().mockResolvedValue(fixtures.game),
  GetGames: vi.fn().mockResolvedValue([fixtures.game]),
  GetLogo: vi.fn().mockResolvedValue(''),
  GetProfile: vi.fn().mockResolvedValue(fixtures.profile),
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

describe('App keyboard behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not run global shortcuts while typing in editable options controls', async () => {
    render(App)

    await waitFor(() => expect(screen.getByText('Options')).toBeTruthy())
    await fireEvent.click(screen.getByText('Options'))

    const steamPath = screen.getAllByPlaceholderText('(default)')[0]
    await fireEvent.input(steamPath, { target: { value: '/mnt/steam' } })
    await fireEvent.keyDown(steamPath, { key: 'q' })

    expect(Quit).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog', { name: 'Options' })).toBeTruthy()
  })

  it('keeps help and quit global shortcuts active outside editable controls', async () => {
    render(App)

    await waitFor(() => expect(screen.getByText('Select a game from the list')).toBeTruthy())
    await fireEvent.keyDown(window, { key: '?' })

    expect(screen.getByRole('dialog', { name: 'Help' })).toBeTruthy()

    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Help' })).toBeNull()

    await fireEvent.keyDown(window, { key: 'q' })
    expect(Quit).toHaveBeenCalledTimes(1)
  })
})
