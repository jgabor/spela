import { describe, expect, it } from 'vitest'

import { rrPresetOptions, srPresetOptions, vrrOptions, emptyProfile, profilePropertyByField } from './profileFieldOptions.js'

const modelPresets = ['A', 'B', 'C', 'D', 'E', 'F', 'J', 'K', 'L', 'M']

it('offers four VRR policies separately from reset and omission', () => {
  expect(vrrOptions.map(option => option.value)).toEqual(['unset', 'automatic', 'always', 'never'])
  expect(emptyProfile().vrr).toBe('')
  expect(profilePropertyByField['gpu.vrr']).toBe('vrr')
})

describe('DLSS preset option parity', () => {
  it('exposes every SR descriptor value with an explicit driver default', () => {
    expect(srPresetOptions.map(option => option.value)).toEqual(['', 'default', 'auto', ...modelPresets])
    expect(srPresetOptions).toContainEqual({ value: 'default', label: 'Driver default' })
  })

  it('exposes every RR descriptor value without the invalid auto preset', () => {
    expect(rrPresetOptions.map(option => option.value)).toEqual(['', 'default', ...modelPresets])
    expect(rrPresetOptions).toContainEqual({ value: 'default', label: 'Driver default' })
    expect(rrPresetOptions.some(option => option.value === 'auto')).toBe(false)
  })
})
