import { test as base } from '@playwright/test'
import { profilePropertyByField } from '../src/lib/profileFieldOptions.js'

export const games = [
  {
    appId: 1091500,
    name: 'Cyberpunk 2077',
    installDir: '/home/user/.steam/steam/steamapps/common/Cyberpunk 2077',
    prefixPath: '/home/user/.steam/steam/steamapps/compatdata/1091500/pfx',
    dlls: [
      { dllType: 'dlss', name: 'nvngx_dlss.dll', path: 'bin/x64/nvngx_dlss.dll', version: '3.7.0' },
      { dllType: 'dlssg', name: 'nvngx_dlssg.dll', path: 'bin/x64/nvngx_dlssg.dll', version: '3.7.0' },
    ],
    hasProfile: true,
  },
  {
    appId: 292030,
    name: 'The Witcher 3: Wild Hunt',
    installDir: '/home/user/.steam/steam/steamapps/common/The Witcher 3',
    prefixPath: '/home/user/.steam/steam/steamapps/compatdata/292030/pfx',
    dlls: [{ dllType: 'dlss', name: 'nvngx_dlss.dll', path: 'bin/nvngx_dlss.dll', version: '3.5.0' }],
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

export const defaultProfile = {
  clockOffset: 0,
  memoryOffset: 0,
  vrr: '',
  preset: 'balanced',
  srMode: 'balanced',
  srOverride: false,
  fgEnabled: false,
  fgOverride: false,
  enableHdr: false,
  enableWayland: false,
  enableNgxUpdater: false,
}

export const config = {
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
  preferredDLLSource: 'techpowerup',
}

export const settingsCatalog = [
  {
    id: 0,
    title: 'Display',
    options: [
      { key: 'theme', label: 'Theme', description: 'Choose the application theme.', type: 'select', choices: ['default', 'dark', 'light'] },
      { key: 'showHints', label: 'Show hints', description: 'Show keyboard hints.', type: 'toggle', choices: ['true', 'false'] },
      { key: 'compactMode', label: 'Compact mode', description: 'Use compact spacing.', type: 'toggle', choices: ['true', 'false'] },
      { key: 'confirmDestructive', label: 'Confirm destructive', description: 'Confirm destructive actions.', type: 'toggle', choices: ['true', 'false'] },
    ],
  },
  { id: 1, title: 'Startup', options: [{ key: 'rescanOnStartup', label: 'Re-scan on startup', description: 'Scan at startup.', type: 'toggle', choices: ['true', 'false'] }] },
  { id: 2, title: 'Paths', options: [{ key: 'steamPath', label: 'Steam path', description: 'Custom Steam path.', type: 'path' }] },
  { id: 3, title: 'DLL policy', options: [{ key: 'manifestRefreshHours', label: 'Refresh interval', description: 'Manifest refresh interval.', type: 'select', choices: ['1', '6', '12', '24', '48', '168'] }] },
  { id: 4, title: 'Logging', options: [{ key: 'logLevel', label: 'Log level', description: 'Logging verbosity.', type: 'select', choices: ['debug', 'info', 'warn', 'error'] }] },
]

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
    const defaultProfile = ${JSON.stringify(mockData.defaultProfile)};
    const profileProperties = ${JSON.stringify(profilePropertyByField)};
    const applyProfilePatch = (target, patch) => {
      const property = profileProperties[patch.field];
      target[property] = patch.operation === 'reset' ? (typeof target[property] === 'number' ? 0 : typeof target[property] === 'boolean' ? false : '') : patch.value;
    };
    window.__profilePatchCalls = [];
    let config = ${JSON.stringify(mockData.config)};
    const settingsCatalog = ${JSON.stringify(mockData.settingsCatalog)};
    const dllUpdates = ${JSON.stringify(mockData.dllUpdates)};

    const defaultNav = {
      destination: 0,
      scopeGlobal: true,
      gameName: '',
      aspect: 1,
      subsystem: 0,
      dllSection: 0,
      monitorSection: 0,
      settingsSection: 0,
      breadcrumb: ['Library', 'All games', 'Profile', 'Proton'],
    };
    const cloneNav = (state) => ({ ...state, breadcrumb: [...(state.breadcrumb || [])] });

    window.go = {
      gui: {
        App: {
          GetConfig: async () => config,
          GetSettingsCatalog: async () => settingsCatalog,
          DefaultNavState: async () => cloneNav(defaultNav),
          NavSelectDestination: async (state, destination) => cloneNav({ ...state, destination }),
          NavSelectScope: async (state, scopeGlobal, gameName = '') => cloneNav({ ...state, scopeGlobal, gameName, aspect: scopeGlobal ? 1 : state.aspect }),
          NavSelectAspect: async (state, aspect) => cloneNav({ ...state, aspect }),
          NavSelectSubsystem: async (state, subsystem) => cloneNav({ ...state, subsystem }),
          NavSelectDLLSection: async (state, section) => cloneNav({ ...state, dllSection: section }),
          NavSelectMonitorSection: async (state, section) => cloneNav({ ...state, monitorSection: section }),
          NavSelectSettingsSection: async (state, section) => cloneNav({ ...state, settingsSection: section }),

          SaveConfig: async (nextConfig) => {
            config = nextConfig;
          },
          SaveConfigOption: async (key, value) => {
            if (typeof config[key] === 'boolean') {
              config[key] = value === 'true';
            } else if (typeof config[key] === 'number') {
              config[key] = Number(value);
            } else {
              config[key] = value;
            }
          },
          GetVersion: async () => '0.5.1',
          GetGames: async () => games,
          GetGame: async (appId) => games.find((g) => g.appId === appId) || null,
          ScanGames: async () => {},
          GetDefaultProfile: async () => defaultProfile,
          PatchDefaultProfile: async (patches) => {
            window.__profilePatchCalls.push({ scope: 'default', patches: structuredClone(patches) });
            patches.forEach((patch) => applyProfilePatch(defaultProfile, patch));
            return defaultProfile;
          },
          GetProfile: async (appId) => profiles[appId] || null,
          PatchProfile: async (appId, patches) => {
            window.__profilePatchCalls.push({ scope: 'game', appId, patches: structuredClone(patches) });
            profiles[appId] ||= structuredClone(defaultProfile);
            patches.forEach((patch) => applyProfilePatch(profiles[appId], patch));
            return profiles[appId];
          },
          GetGPUInfo: async () => gpuInfo,
          GetCPUInfo: async () => cpuInfo,
          CheckDLLUpdates: async (appId) => dllUpdates[appId] || [],
          UpdateDLLs: async () => {},
          RestoreDLLs: async () => {},
          HasDLLBackup: async () => false,
          ListDLLInstallTypes: async () => ['dlss', 'dlssg'],
          ListDLLVersions: async () => ['3.8.0', '3.7.0'],
          InstallDLL: async () => {},
          LaunchGame: async () => {
            throw new Error('direct Steam URI launch cannot track the game lifetime; set Steam launch options to spela %command% instead');
          },
          VKD3DHeapCompatibilityNotice: async () => '',
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
    const mockData = { games, gpuInfo, cpuInfo, profiles, defaultProfile, config, settingsCatalog, dllUpdates }
    await page.addInitScript(createMockScript(mockData))
    await use(page)
  },
})

export { expect } from '@playwright/test'
