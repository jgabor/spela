import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const fixtures = vi.hoisted(() => ({
  games: [
    {
      appId: 1091500,
      name: 'Cyberpunk 2077',
      installDir: '/games/cyberpunk',
      hasProfile: true,
      dlls: [{ dllType: 'dlss', version: '3.8.10' }]
    },
    {
      appId: 1245620,
      name: 'Elden Ring',
      installDir: '/games/elden-ring',
      hasProfile: false,
      dlls: []
    },
    {
      appId: 620,
      name: 'Portal 2',
      installDir: '/games/portal2',
      hasProfile: false,
      dlls: [{ dllType: 'dlss', version: '3.7.0' }]
    },
    {
      appId: 292030,
      name: 'The Witcher 3',
      installDir: '/games/witcher3',
      hasProfile: true,
      dlls: []
    }
  ]
}))

import GameList from './GameList.svelte'
import { applyFiltersAndSort } from './gameListBehavior'

function desktop(overrides = {}) {
  return {
    GetGames: vi.fn().mockResolvedValue(fixtures.games),
    ScanGames: vi.fn().mockResolvedValue(undefined),
    UpdateDLLs: vi.fn().mockResolvedValue(undefined),
    ...overrides
  }
}

function gameNames(list) {
  return list.map(game => game.name)
}

describe('GameList current behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('renders the default profile, game badges, search filtering, and the filtered empty state', async () => {
    render(GameList, { props: { desktop: desktop() } })

    expect(screen.getByText('Loading...')).toBeTruthy()

    await waitFor(() => {
      expect(screen.getByText('All games (default)')).toBeTruthy()
      expect(screen.getByText('Cyberpunk 2077')).toBeTruthy()
      expect(screen.getByText('Elden Ring')).toBeTruthy()
      expect(screen.getByText('Portal 2')).toBeTruthy()
      expect(screen.getByText('The Witcher 3')).toBeTruthy()
      expect(screen.getByText('4 games')).toBeTruthy()
    })
    expect(screen.getAllByText('● DLLs')).toHaveLength(3)
    expect(screen.getAllByText('◆ Profile')).toHaveLength(3)

    await fireEvent.input(screen.getByPlaceholderText('Search games...'), { target: { value: 'cyber' } })

    expect(screen.getByText('Cyberpunk 2077')).toBeTruthy()
    expect(screen.queryByText('Elden Ring')).toBeNull()
    expect(screen.getByText('1 game')).toBeTruthy()

    await fireEvent.input(screen.getByPlaceholderText('Search games...'), { target: { value: 'missing' } })

    expect(screen.getByText('No games matching filters')).toBeTruthy()
    expect(screen.queryByText('All games (default)')).toBeNull()
  })

  it('sorts DLL games first and profile games first without changing current badge meanings', async () => {
    const { container } = render(GameList, { props: { desktop: desktop() } })

    await waitFor(() => expect(screen.getByText('4 games')).toBeTruthy())

    await fireEvent.click(screen.getByText('Name A-Z'))
    await fireEvent.click(screen.getByText('DLLs first'))

    let names = [...container.querySelectorAll('.game-item:not(.default-profile) .name')].map(node => node.textContent)
    expect(names).toEqual(['Cyberpunk 2077', 'Portal 2', 'Elden Ring', 'The Witcher 3'])

    await fireEvent.click(screen.getByText('DLLs first'))
    await fireEvent.click(screen.getByText('Profile first'))

    names = [...container.querySelectorAll('.game-item:not(.default-profile) .name')].map(node => node.textContent)
    expect(names).toEqual(['Cyberpunk 2077', 'The Witcher 3', 'Elden Ring', 'Portal 2'])
  })

  it('skips selected games without DLLs during batch update and shows mixed failures', async () => {
    const replacementDesktop = desktop({
      UpdateDLLs: vi.fn().mockRejectedValueOnce(new Error('network offline'))
    })
    render(GameList, { props: { desktop: replacementDesktop } })

    await waitFor(() => expect(screen.getByText('4 games')).toBeTruthy())
    await fireEvent.click(screen.getByText('Select'))
    await fireEvent.click(screen.getByText('Select all'))
    await fireEvent.click(screen.getByText('Update all DLLs'))

    await waitFor(() => expect(screen.getByText('Select')).toBeTruthy())
    expect(replacementDesktop.UpdateDLLs).toHaveBeenCalledTimes(2)
    expect(replacementDesktop.UpdateDLLs).toHaveBeenCalledWith(1091500)
    expect(replacementDesktop.UpdateDLLs).toHaveBeenCalledWith(620)
    expect(screen.getByText('Updated 1 game, 1 failed, 2 skipped')).toBeTruthy()
  })
})

describe('game list behavior decisions', () => {
  it('filters by search, DLL state, and profile state before sorting by name', () => {
    const visible = applyFiltersAndSort(fixtures.games, 'cyber', true, true, 'name-asc')

    expect(gameNames(visible)).toEqual(['Cyberpunk 2077'])
  })

  it('sorts profile games first and breaks ties by name', () => {
    const visible = applyFiltersAndSort(fixtures.games, '', false, false, 'profile-first')

    expect(gameNames(visible)).toEqual(['Cyberpunk 2077', 'The Witcher 3', 'Elden Ring', 'Portal 2'])
  })
})
