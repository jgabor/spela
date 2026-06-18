/** Mirrors internal/nav — shared navigation model for GUI. */

export const Destination = {
  Library: 0,
  DLLCatalog: 1,
  Monitor: 2,
  Settings: 3
}

export const DestinationLabels = ['Library', 'DLL Catalog', 'Monitor', 'Settings']

export const SettingsSectionLabels = ['Display', 'Startup', 'Paths', 'DLL policy', 'Logging']

export const MonitorSectionLabels = ['GPU', 'CPU', 'Alerts']

export function destinationFromHotkey(key) {
  switch (key) {
    case '1':
      return Destination.Library
    case '2':
      return Destination.DLLCatalog
    case '3':
      return Destination.Monitor
    case '4':
      return Destination.Settings
    default:
      return null
  }
}

export const Aspect = {
  Overview: 0,
  Profile: 1,
  DLLs: 2
}

export const AspectLabels = ['Overview', 'Profile', 'DLLs']

export const ProfileSubsystemLabels = ['Proton', 'DLSS', 'GPU', 'CPU', 'Overlay']

export function defaultNavState() {
  return {
    destination: Destination.Library,
    scopeGlobal: true,
    gameName: '',
    aspect: Aspect.Profile,
    subsystem: 0,
    dllSection: 0,
    monitorSection: 0,
    settingsSection: 0
  }
}

export function selectDestination(state, destination) {
  const next = { ...state, destination }
  switch (destination) {
    case Destination.Library:
      if (next.scopeGlobal) {
        next.aspect = Aspect.Profile
      } else {
        next.aspect = Aspect.Overview
      }
      break
    case Destination.DLLCatalog:
      next.dllSection = 0
      break
    case Destination.Monitor:
      next.monitorSection = 0
      break
    case Destination.Settings:
      next.settingsSection = 0
      break
  }
  return next
}

export function selectScope(state, scopeGlobal, gameName = '') {
  const next = { ...state, scopeGlobal, gameName }
  if (scopeGlobal) {
    next.aspect = Aspect.Profile
    return next
  }
  if (next.aspect !== Aspect.Profile && next.aspect !== Aspect.DLLs) {
    next.aspect = Aspect.Overview
  }
  return next
}

export function selectAspect(state, aspect) {
  if (state.scopeGlobal && (aspect === Aspect.Overview || aspect === Aspect.DLLs)) {
    return state
  }
  return { ...state, aspect }
}

export function selectSubsystem(state, subsystem) {
  return { ...state, subsystem }
}

export function breadcrumb(state) {
  const parts = [DestinationLabels[state.destination] ?? 'Spela']
  if (state.destination === Destination.Library) {
    parts.push(state.scopeGlobal ? 'All games' : state.gameName || 'Game')
    parts.push(AspectLabels[state.aspect] ?? 'Profile')
    if (state.aspect === Aspect.Profile) {
      parts.push(ProfileSubsystemLabels[state.subsystem] ?? 'Proton')
    }
  } else if (state.destination === Destination.DLLCatalog) {
    parts.push(state.dllSection === 1 ? 'Deployment' : 'Library')
  } else if (state.destination === Destination.Monitor) {
    parts.push(MonitorSectionLabels[state.monitorSection] ?? 'GPU')
  } else if (state.destination === Destination.Settings) {
    parts.push(SettingsSectionLabels[state.settingsSection] ?? 'Display')
  }
  return parts
}
