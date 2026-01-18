import { test, expect, games, gpuInfo, cpuInfo } from './fixtures.js'

test.describe('navigation', () => {
  test('shows navigation buttons', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('button', { name: 'Games' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Monitor' })).toBeVisible()
  })

  test('games view is active by default', async ({ page }) => {
    await page.goto('/')
    const gamesButton = page.getByRole('button', { name: 'Games' })
    await expect(gamesButton).toHaveClass(/active/)
  })
})

test.describe('game list', () => {
  test('shows game list on startup', async ({ page }) => {
    await page.goto('/')
    for (const game of games) {
      await expect(page.getByText(game.name)).toBeVisible()
    }
  })

  test('shows DLSS badge for games with DLLs', async ({ page }) => {
    await page.goto('/')
    const cyberpunkItem = page.locator('.game-item', { hasText: 'Cyberpunk 2077' })
    await expect(cyberpunkItem.locator('.badge.dlss')).toBeVisible()
  })

  test('shows Profile badge for games with profiles', async ({ page }) => {
    await page.goto('/')
    const cyberpunkItem = page.locator('.game-item', { hasText: 'Cyberpunk 2077' })
    await expect(cyberpunkItem.locator('.badge.profile')).toBeVisible()
  })

  test('filters games by search', async ({ page }) => {
    await page.goto('/')
    await page.getByPlaceholder('Search games...').fill('Cyber')
    await expect(page.getByText('Cyberpunk 2077')).toBeVisible()
    await expect(page.getByText('The Witcher 3')).not.toBeVisible()
    await expect(page.getByText('Elden Ring')).not.toBeVisible()
  })

  test('shows empty state when no matches', async ({ page }) => {
    await page.goto('/')
    await page.getByPlaceholder('Search games...').fill('nonexistent game')
    await expect(page.getByText(/No games matching/)).toBeVisible()
  })

  test('clicking a game shows game detail', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h1', { hasText: 'Cyberpunk 2077' })).toBeVisible()
    const appIdRow = page.locator('.row', { hasText: 'App ID' })
    await expect(appIdRow.locator('.value')).toHaveText('1091500')
  })
})

test.describe('monitor view', () => {
  test('shows GPU info', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Monitor' }).click()
    await expect(page.getByText(gpuInfo.name)).toBeVisible()
    await expect(page.getByText(`${gpuInfo.temperature}°C`)).toBeVisible()
  })

  test('shows CPU info', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Monitor' }).click()
    await expect(page.getByText(cpuInfo.model)).toBeVisible()
    await expect(page.getByText(String(cpuInfo.cores))).toBeVisible()
  })
})

test.describe('navigation flow', () => {
  test('back button returns to game list', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h1', { hasText: 'Cyberpunk 2077' })).toBeVisible()
    await page.getByRole('button', { name: '← Back' }).click()
    await expect(page.getByPlaceholder('Search games...')).toBeVisible()
  })

  test('clicking Games nav returns from Monitor', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Monitor' }).click()
    await expect(page.getByText(gpuInfo.name)).toBeVisible()
    await page.getByRole('button', { name: 'Games' }).click()
    await expect(page.getByPlaceholder('Search games...')).toBeVisible()
  })
})
