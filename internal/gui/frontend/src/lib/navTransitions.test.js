import { beforeEach, describe, expect, it, vi } from 'vitest'

const desktop = vi.hoisted(() => ({
  DefaultNavState: vi.fn(),
  NavSelectDestination: vi.fn(),
  NavSelectScope: vi.fn(),
  NavSelectAspect: vi.fn(),
  NavSelectSubsystem: vi.fn(),
  NavSelectDLLSection: vi.fn(),
  NavSelectMonitorSection: vi.fn(),
  NavSelectSettingsSection: vi.fn()
}))

vi.mock('./desktop.js', () => ({ desktopCommands: desktop }))

import {
  breadcrumb, defaultNavState, selectAspect, selectDestination, selectDLLSection,
  selectMonitorSection, selectScope, selectSettingsSection, selectSubsystem
} from './navState.js'

describe('navigation transition adapter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    for (const [name, fn] of Object.entries(desktop)) {
      if (name === 'DefaultNavState') continue
      fn.mockImplementation(async (...args) => ({ operation: name, args }))
    }
  })

  it('routes every supported transition through the desktop contract', async () => {
    const state = { destination: 0, breadcrumb: ['Library'] }
    const calls = [
      [selectDestination, [state, 1], 'NavSelectDestination'],
      [selectScope, [state, false, 'Cyberpunk'], 'NavSelectScope'],
      [selectAspect, [state, 2], 'NavSelectAspect'],
      [selectSubsystem, [state, 3], 'NavSelectSubsystem'],
      [selectDLLSection, [state, 1], 'NavSelectDLLSection'],
      [selectMonitorSection, [state, 2], 'NavSelectMonitorSection'],
      [selectSettingsSection, [state, 4], 'NavSelectSettingsSection']
    ]
    for (const [transition, args, operation] of calls) {
      await expect(transition(...args)).resolves.toEqual({ operation, args })
    }
    expect(breadcrumb(state)).toEqual(['Library'])
    expect(breadcrumb({})).toEqual([])
  })

  it('uses backend and generated default state paths', async () => {
    desktop.DefaultNavState.mockResolvedValue({ destination: 3 })
    await expect(defaultNavState()).resolves.toEqual({ destination: 3 })
    desktop.DefaultNavState = null
    await expect(defaultNavState()).resolves.toMatchObject({ destination: 0, scopeGlobal: true })
  })
})
