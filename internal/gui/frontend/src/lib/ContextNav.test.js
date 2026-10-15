import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import ContextNav from './ContextNav.svelte'
import { Aspect } from './navState.js'

const desktop = () => ({ GetGames: vi.fn().mockResolvedValue([]) })

describe('ContextNav tabs', () => {
  it('offers Profile and DLLs for a game and no Overview', () => {
    render(ContextNav, { props: { desktop: desktop(), scopeGlobal: false, selectedGame: { name: 'Alpha' }, aspect: Aspect.Profile } })
    const tabs = screen.getAllByRole('tab').map(tab => tab.textContent.trim())
    expect(tabs).toEqual(['1 Profile', '2 DLLs'])
  })

  it('offers only Profile on All games', () => {
    render(ContextNav, { props: { desktop: desktop(), scopeGlobal: true, aspect: Aspect.Profile } })
    expect(screen.getAllByRole('tab').map(tab => tab.textContent.trim())).toEqual(['1 Profile'])
  })
})
