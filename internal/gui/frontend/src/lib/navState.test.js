import { describe, it, expect } from 'vitest'
import {
  defaultNavState,
  Destination,
  Aspect,
  selectDestination,
  selectScope,
  selectAspect
} from './navState.js'

describe('navState transitions', () => {
  it('selectDestination resets DLL catalog section', () => {
    const state = defaultNavState()
    const next = selectDestination(state, Destination.DLLCatalog)
    expect(next.destination).toBe(Destination.DLLCatalog)
    expect(next.dllSection).toBe(0)
    expect(next.monitorSection).toBe(0)
    expect(next.settingsSection).toBe(0)
  })

  it('selectDestination resets game scope to overview when returning to Library', () => {
    const state = {
      ...defaultNavState(),
      scopeGlobal: false,
      gameName: 'Cyberpunk 2077',
      aspect: Aspect.DLLs
    }
    let next = selectDestination(state, Destination.Monitor)
    next = selectDestination(next, Destination.Library)
    expect(next.aspect).toBe(Aspect.Overview)
  })

  it('selectScope global forces profile aspect', () => {
    const state = { ...defaultNavState(), scopeGlobal: false, aspect: Aspect.DLLs }
    const next = selectScope(state, true)
    expect(next.scopeGlobal).toBe(true)
    expect(next.aspect).toBe(Aspect.Profile)
  })

  it('selectAspect blocks overview for global scope', () => {
    const state = defaultNavState()
    const next = selectAspect(state, Aspect.Overview)
    expect(next.aspect).toBe(Aspect.Profile)
  })
})
