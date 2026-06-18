/** Nav constants are generated from internal/nav; transitions call Wails bindings. */
export * from './navContract.generated.js'

import { desktopCommands as desktop } from './desktop.js'
import { initialNavState } from './navContract.generated.js'

export async function defaultNavState() {
  if (desktop.DefaultNavState) {
    return desktop.DefaultNavState()
  }
  return initialNavState()
}

export async function selectDestination(state, destination) {
  return desktop.NavSelectDestination(state, destination)
}

export async function selectScope(state, scopeGlobal, gameName = '') {
  return desktop.NavSelectScope(state, scopeGlobal, gameName)
}

export async function selectAspect(state, aspect) {
  return desktop.NavSelectAspect(state, aspect)
}

export async function selectSubsystem(state, subsystem) {
  return desktop.NavSelectSubsystem(state, subsystem)
}

export async function selectDLLSection(state, section) {
  return desktop.NavSelectDLLSection(state, section)
}

export async function selectMonitorSection(state, section) {
  return desktop.NavSelectMonitorSection(state, section)
}

export async function selectSettingsSection(state, section) {
  return desktop.NavSelectSettingsSection(state, section)
}

export function breadcrumb(state) {
  return state.breadcrumb ?? []
}
