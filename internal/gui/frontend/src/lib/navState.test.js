import { describe, it, expect } from 'vitest'
import {
  Destination,
  Aspect,
  DestinationLabels,
  destinationFromHotkey,
  initialNavState
} from './navContract.generated.js'

describe('generated nav contract', () => {
  it('exports canonical destination enums', () => {
    expect(Destination.Library).toBe(0)
    expect(Destination.DLLCatalog).toBe(1)
    expect(Destination.Monitor).toBe(2)
    expect(Destination.Settings).toBe(3)
  })

  it('exports destination labels from Go', () => {
    expect(DestinationLabels).toEqual(['Library', 'DLL Catalog', 'Monitor', 'Settings'])
  })

  it('maps digit hotkeys to destinations', () => {
    expect(destinationFromHotkey('2')).toBe(Destination.DLLCatalog)
    expect(destinationFromHotkey('x')).toBeNull()
  })

  it('seeds initial nav state with breadcrumb', () => {
    const state = initialNavState()
    expect(state.scopeGlobal).toBe(true)
    expect(state.aspect).toBe(Aspect.Profile)
    expect(state.breadcrumb.length).toBeGreaterThan(0)
  })
})
