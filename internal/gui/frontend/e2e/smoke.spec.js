import { test, expect, games } from './fixtures.js'

test.describe('shell', () => {
  test('shows the current resource shell', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByPlaceholder('Search games...')).toBeVisible()
    await expect(page.getByText('Default profile')).toBeVisible()
    await expect(page.getByText('NVIDIA DLSS super resolution and frame generation settings')).toBeVisible()
  })

  test('opens options and help overlays', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Options' }).click()
    await expect(page.getByRole('dialog', { name: 'Options' })).toBeVisible()
    await page.keyboard.press('Escape')
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

  test('clicking a game shows game detail', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h1', { hasText: 'Cyberpunk 2077' })).toBeVisible()
    const appIdRow = page.locator('.row', { hasText: 'App ID' })
    await expect(appIdRow.locator('.value')).toHaveText('1091500')
  })
})

test.describe('navigation flow', () => {
  test('default profile entry shows default profile detail', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Default profile').click()
    await expect(page.locator('h1', { hasText: 'Default profile' })).toBeVisible()
    await expect(page.getByText('Applies to games without their own profile.')).toBeVisible()
  })
})
