import { render, screen } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetGames: vi.fn().mockImplementation(() => new Promise(() => {})),
  ScanGames: vi.fn().mockImplementation(() => new Promise(() => {})),
}))

import GameList from './GameList.svelte'

describe('GameList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading state initially', () => {
    render(GameList)
    expect(screen.getByText('Loading...')).toBeTruthy()
  })

  it('renders search input', () => {
    render(GameList)
    expect(screen.getByPlaceholderText('Search games...')).toBeTruthy()
  })

  it('renders rescan button', () => {
    render(GameList)
    expect(screen.getByText('Rescan')).toBeTruthy()
  })

  it('has the correct structure', () => {
    const { container } = render(GameList)
    expect(container.querySelector('.game-list')).toBeTruthy()
    expect(container.querySelector('.header')).toBeTruthy()
  })
})
