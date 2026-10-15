import { test as base } from '@playwright/test'
import { profilePropertyByField } from '../src/lib/profileFieldOptions.js'
import { fieldSemantics } from '../src/lib/testSemantics.js'

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
  utilizationPercent: 12,
  memoryUsedMegabytes: 16384,
  memoryTotalMegabytes: 32768,
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
    clockOffset: 100,
    memoryOffset: 0,
    powerLimit: 0,
    fanSpeed: 0,
    affinity: '',
  },
}

// The settings each game holds itself. The mock backend derives the sources, the
// action words, and the whole-page Follow from these, as the real one does.
export const ownFields = {
  default: ['proton.enable_hdr'],
  1091500: ['dlss.sr_mode', 'dlss.fg_enabled', 'dlss.fg_override', 'proton.enable_hdr', 'gpu.clock_offset'],
}

export const defaultProfile = {
  clockOffset: 0,
  memoryOffset: 0,
  powerLimit: 0,
  fanSpeed: 0,
  affinity: '',
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
    // The real backend sends the shared words with every projection.
    const gameWords = ${JSON.stringify(fieldSemantics())};
    const allGamesWords = ${JSON.stringify(fieldSemantics({}, { allGames: true }))};
    const profileProperties = ${JSON.stringify(profilePropertyByField)};
    const owns = ${JSON.stringify(mockData.ownFields)};
    const defaultOwn = new Set(owns.default || []);
    const ownSet = (appId) => (owns[appId] = new Set(owns[appId] || []));
    Object.keys(owns).filter((key) => key !== 'default').forEach((appId) => ownSet(appId));
    const zero = (value) => (typeof value === 'number' ? 0 : typeof value === 'boolean' ? false : '');
    const applyProfilePatch = (target, own, patch) => {
      const property = profileProperties[patch.field];
      if (patch.operation === 'reset') {
        target[property] = zero(target[property]);
        own.delete(patch.field);
      } else if (patch.operation === 'pin') {
        own.add(patch.field);
      } else {
        target[property] = patch.value;
        own.add(patch.field);
      }
    };
    const valueText = (word, value) => {
      if (typeof value === 'boolean') return value ? 'On' : 'Off';
      if (typeof value === 'number') return word.unit ? value + (word.unit === '%' ? '' : ' ') + word.unit : String(value);
      return value === '' || value === undefined ? 'none' : String(value);
    };
    // The games that set their own value for a field, by name.
    const gamesWith = (fields) => games
      .filter((entry) => fields.some((field) => ownSet(entry.appId).has(field)))
      .map((entry) => entry.name)
      .sort();
    // project mirrors the real projection: sources, action words, value text, and
    // the whole-page Follow all follow from what the page holds itself.
    const project = (profile, own, allGames) => {
      const base = allGames ? allGamesWords : gameWords;
      const semantics = base.map((word) => {
        const isOwn = own.has(word.field);
        const owners = allGames ? gamesWith([word.field]).length : 0;
        return {
          ...word,
          ownValueSuffix: owners ? (owners === 1 ? '1 game has its own value' : owners + ' games have their own value') : '',
          ownValueCount: owners,
          ownValueAction: owners ? 'Apply to games with their own value' : '',
          source: isOwn ? 'override' : 'default',
          origin: allGames ? (isOwn ? 'set_here' : 'system_default') : (isOwn ? 'this_game' : 'all_games'),
          sourceText: allGames ? (isOwn ? 'set here' : 'system default') : (isOwn ? 'this game · All games: Off' : 'from All games'),
          action: allGames ? (isOwn ? 'Use system default' : 'Set here') : (isOwn ? 'Follow All games' : 'Set for this game'),
          valueText: valueText(word, profile[profileProperties[word.field]]),
        };
      });
      const count = own.size;
      return {
        ...profile,
        semantics,
        clearAll: {
          action: allGames ? 'Use system default for all settings' : 'Follow All games for all settings',
          count,
          body: count === 0 ? 'Nothing to clear.' : 'This clears the ' + count + ' setting' + (count === 1 ? '' : 's') + ' that ' + (allGames ? 'All games sets' : 'this game sets') + ' itself. It is saved at once.',
        },
      };
    };
    window.__profilePatchCalls = [];
    window.__applyPreviews = [];
    window.__applyCalls = [];
    window.__failNextPatch = '';
    window.__patchDelay = 0;
    const settle = async () => {
      if (window.__patchDelay) await new Promise((resolve) => setTimeout(resolve, window.__patchDelay));
      if (window.__failNextPatch) {
        const reason = window.__failNextPatch;
        window.__failNextPatch = '';
        throw new Error(reason);
      }
    };
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
          NavSelectScope: async (state, scopeGlobal, gameName = '') => cloneNav({ ...state, scopeGlobal, gameName, aspect: scopeGlobal || state.aspect !== 2 ? 1 : state.aspect }),
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
          GetLogo: async () => '',
          GetGames: async () => games,
          GetGame: async (appId) => games.find((g) => g.appId === appId) || null,
          ScanGames: async () => {},
          GetDefaultProfile: async () => project(defaultProfile, defaultOwn, true),
          PatchDefaultProfile: async (patches) => {
            window.__profilePatchCalls.push({ scope: 'default', patches: structuredClone(patches) });
            await settle();
            patches.forEach((patch) => applyProfilePatch(defaultProfile, defaultOwn, patch));
            return project(defaultProfile, defaultOwn, true);
          },
          // A game without a profile reads as All games, as the real backend projects it.
          GetProfile: async (appId) => profiles[appId]
            ? project(profiles[appId], ownSet(appId), false)
            : { ...project(structuredClone(defaultProfile), new Set(), false), inheritedFromDefault: true },
          PatchProfile: async (appId, patches) => {
            window.__profilePatchCalls.push({ scope: 'game', appId, patches: structuredClone(patches) });
            await settle();
            profiles[appId] ||= structuredClone(defaultProfile);
            const own = ownSet(appId);
            patches.forEach((patch) => applyProfilePatch(profiles[appId], own, patch));
            const game = games.find((entry) => entry.appId === appId);
            if (game) game.hasProfile = true;
            return project(profiles[appId], own, false);
          },
          PreviewApplyToGamesWithOwnValue: async (fields) => {
            window.__applyPreviews.push(structuredClone(fields));
            const names = gamesWith(fields);
            const word = gameWords.find((entry) => entry.field === fields[0]);
            const value = fields.length === 1 ? ' (' + valueText(word, defaultProfile[profileProperties[fields[0]]]) + ')' : '';
            return {
              action: 'Apply to games with their own value',
              count: names.length,
              appIds: games.filter((entry) => fields.some((field) => ownSet(entry.appId).has(field))).map((entry) => entry.appId),
              body: word.label + ': ' + names.length + (names.length === 1 ? ' game has' : ' games have') + ' their own value: ' + names.join(', ') + '. ' + (names.length === 1 ? 'It' : 'They') + ' will follow All games' + value + '. It is saved at once and cannot be undone.',
            };
          },
          ApplyToGamesWithOwnValue: async (fields, appIds) => {
            window.__applyCalls.push(structuredClone(fields));
            await settle();
            const named = games.filter((entry) => appIds.includes(entry.appId));
            const names = named.filter((entry) => fields.some((field) => ownSet(entry.appId).has(field))).map((entry) => entry.name);
            named.forEach((entry) => {
              fields.forEach((field) => {
                if (!ownSet(entry.appId).has(field)) return;
                profiles[entry.appId] ||= structuredClone(defaultProfile);
                applyProfilePatch(profiles[entry.appId], ownSet(entry.appId), { field, operation: 'reset' });
              });
            });
            const word = gameWords.find((entry) => entry.field === fields[0]);
            return {
              message: word.label + ': ' + names.length + (names.length === 1 ? ' game follows' : ' games follow') + ' All games',
              failure: '',
              failed: [],
              profile: project(defaultProfile, defaultOwn, true),
            };
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
    const mockData = { games, gpuInfo, cpuInfo, profiles, ownFields, defaultProfile, config, settingsCatalog, dllUpdates }
    await page.addInitScript(createMockScript(mockData))
    await use(page)
  },
})

export { expect } from '@playwright/test'
