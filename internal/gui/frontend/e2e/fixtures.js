import { test as base } from '@playwright/test'

export const games = [
  {
    appId: 1091500,
    name: 'Cyberpunk 2077',
    installDir: '/home/user/.steam/steam/steamapps/common/Cyberpunk 2077',
    prefixPath: '/home/user/.steam/steam/steamapps/compatdata/1091500/pfx',
    dlls: [
      { name: 'nvngx_dlss.dll', path: 'bin/x64/nvngx_dlss.dll', version: '3.7.0' },
      { name: 'nvngx_dlssg.dll', path: 'bin/x64/nvngx_dlssg.dll', version: '3.7.0' },
    ],
    hasProfile: true,
  },
  {
    appId: 292030,
    name: 'The Witcher 3: Wild Hunt',
    installDir: '/home/user/.steam/steam/steamapps/common/The Witcher 3',
    prefixPath: '/home/user/.steam/steam/steamapps/compatdata/292030/pfx',
    dlls: [{ name: 'nvngx_dlss.dll', path: 'bin/nvngx_dlss.dll', version: '3.5.0' }],
    hasProfile: false,
  },
  {
    appId: 1245620,
    name: 'Elden Ring',
    installDir: '/home/user/.steam/steam/steamapps/common/ELDEN RING',
    prefixPath: '/home/user/.steam/steam/steamapps/compatdata/1245620/pfx',
    dlls: [],
    hasProfile: false,
  },
]

export const gpuInfo = {
  name: 'NVIDIA GeForce RTX 4090',
  temperature: 65,
  powerDraw: 320.5,
  powerLimit: 450.0,
  utilization: 85,
  memoryUsed: 18432,
  memoryTotal: 24576,
  graphicsClock: 2520,
  memoryClock: 10501,
}

export const cpuInfo = {
  model: 'AMD Ryzen 9 7950X',
  cores: 16,
  averageFrequency: 5200,
  governor: 'performance',
  smtEnabled: true,
}

export const profiles = {
  1091500: {
    preset: 'quality',
    srMode: 'balanced',
    srOverride: false,
    fgEnabled: true,
    fgOverride: true,
    enableHdr: true,
    enableWayland: false,
    enableNgxUpdater: false,
  },
}

export const dllUpdates = {
  1091500: [
    { name: 'nvngx_dlss.dll', currentVersion: '3.7.0', latestVersion: '3.8.0', hasUpdate: true },
    { name: 'nvngx_dlssg.dll', currentVersion: '3.7.0', latestVersion: '3.7.0', hasUpdate: false },
  ],
  292030: [{ name: 'nvngx_dlss.dll', currentVersion: '3.5.0', latestVersion: '3.8.0', hasUpdate: true }],
}

function createMockScript(mockData) {
  return `
    const games = ${JSON.stringify(mockData.games)};
    const gpuInfo = ${JSON.stringify(mockData.gpuInfo)};
    const cpuInfo = ${JSON.stringify(mockData.cpuInfo)};
    const profiles = ${JSON.stringify(mockData.profiles)};
    const dllUpdates = ${JSON.stringify(mockData.dllUpdates)};

    window.go = {
      main: {
        App: {
          GetGames: async () => games,
          GetGame: async (appId) => games.find((g) => g.appId === appId) || null,
          ScanGames: async () => {},
          GetProfile: async (appId) => profiles[appId] || null,
          SaveProfile: async (appId, profile) => {
            profiles[appId] = profile;
          },
          GetGPUInfo: async () => gpuInfo,
          GetCPUInfo: async () => cpuInfo,
          CheckDLLUpdates: async (appId) => dllUpdates[appId] || [],
          UpdateDLLs: async () => {},
          RestoreDLLs: async () => {},
          HasDLLBackup: async () => false,
        },
      },
    };

    window.runtime = {
      LogPrint: () => {},
      LogTrace: () => {},
      LogDebug: () => {},
      LogInfo: () => {},
      LogWarning: () => {},
      LogError: () => {},
      LogFatal: () => {},
      EventsOnMultiple: () => () => {},
      EventsOn: () => () => {},
      EventsOff: () => {},
      EventsOffAll: () => {},
      EventsOnce: () => () => {},
      EventsEmit: () => {},
      WindowReload: () => {},
      WindowSetTitle: () => {},
      BrowserOpenURL: () => {},
      Environment: () => ({ platform: 'linux' }),
      Quit: () => {},
    };
  `
}

export const test = base.extend({
  page: async ({ page }, use) => {
    const mockData = { games, gpuInfo, cpuInfo, profiles, dllUpdates }
    await page.addInitScript(createMockScript(mockData))
    await use(page)
  },
})

export { expect } from '@playwright/test'
