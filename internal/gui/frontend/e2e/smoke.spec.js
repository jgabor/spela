import { test, expect, games } from './fixtures.js'

test.describe('shell', () => {
  test('shows the current resource shell', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByPlaceholder('Search games...')).toBeVisible()
    await expect(page.getByText('All games (default)')).toBeVisible()
    await expect(page.getByText('NVIDIA DLSS super resolution and frame generation settings')).toBeVisible()
  })

  test('opens settings destination and help overlay', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Settings', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
    await page.keyboard.press('?')
    await expect(page.getByRole('dialog', { name: 'Help' })).toBeVisible()
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

  test('clicking a game shows library overview', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h1', { hasText: 'Cyberpunk 2077' })).toBeVisible()
    await expect(page.locator('dt', { hasText: 'App ID' }).locator('..').locator('dd')).toHaveText('1091500')
  })
})

test.describe('navigation flow', () => {
  test('default scope entry shows default profile detail', async ({ page }) => {
    await page.goto('/')
    await page.getByText('All games (default)').click()
    await expect(page.locator('h1', { hasText: 'All games (default)' })).toBeVisible()
    await expect(page.getByText('Applies to games without their own profile.')).toBeVisible()
  })

  test('opens DLL catalog destination', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: '[2] DLL Catalog' }).click()
    await expect(page.getByRole('heading', { name: 'DLL Library' })).toBeVisible()
    await page.getByRole('button', { name: 'Deployment' }).click()
    await expect(page.getByRole('heading', { name: 'Deployment matrix' })).toBeVisible()
  })

  test('opens monitor destination', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: '[3] Monitor' }).click()
    await expect(page.getByRole('heading', { name: 'GPU' })).toBeVisible()
    await page.getByRole('button', { name: 'CPU' }).click()
    await expect(page.getByRole('heading', { name: 'CPU' })).toBeVisible()
  })
})
