import { test, expect, games, profiles } from './fixtures.js'

const cyberpunk = games[0]
const witcher = games[1]

test.describe('game detail', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
  })

  test('displays game information', async ({ page }) => {
    await expect(page.locator('h1', { hasText: cyberpunk.name })).toBeVisible()
    const appIdRow = page.locator('.row', { hasText: 'App ID' })
    await expect(appIdRow.locator('.value')).toHaveText(String(cyberpunk.appId))
    await expect(page.getByText(cyberpunk.installDir)).toBeVisible()
  })

  test('displays prefix path', async ({ page }) => {
    await expect(page.getByText(cyberpunk.prefixPath)).toBeVisible()
  })
})

test.describe('DLL section', () => {
  test('shows detected DLLs heading', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h2', { hasText: 'DLL versions' })).toBeVisible()
  })

  test('displays DLL names and versions', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('.dll-row').nth(1)).toContainText('3.7.0')
  })

  test('shows update badge for DLLs with updates', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByRole('button', { name: 'Update all DLLs' })).toBeVisible()
  })

  test('does not show DLL section for games without DLLs', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Elden Ring').click()
    await expect(page.locator('h2', { hasText: 'DLL versions' })).toBeVisible()
    await expect(page.locator('.dll-row').nth(1)).toContainText('-')
  })
})

test.describe('profile settings', () => {
  test('shows profile settings sections', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h2', { hasText: 'DLSS settings' })).toBeVisible()
    await expect(page.locator('h2', { hasText: 'Proton settings' })).toBeVisible()
  })

  test('displays quality mode dropdown', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const qualityField = page.locator('.field', { hasText: 'Quality mode' })
    await expect(qualityField.locator('.trigger')).toContainText('Balanced')
  })

  test('displays DLSS-SR mode dropdown', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const presetField = page.locator('.field', { hasText: 'DLSS preset' })
    await expect(presetField.locator('.trigger')).toBeVisible()
  })

  test('displays checkbox options', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByLabel('Override (force DLSS even if unsupported)')).toBeVisible()
    const frameGenField = page.locator('.field', { hasText: 'Frame generation' }).first()
    await expect(frameGenField.locator('.trigger')).toBeVisible()
    await expect(page.getByLabel('HDR')).toBeVisible()
    await expect(page.getByLabel('Wayland')).toBeVisible()
    await expect(page.getByLabel('NGX Updater')).toBeVisible()
  })

  test('checkbox values match profile', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const profile = profiles[1091500]
    const frameGenField = page.locator('.field', { hasText: 'Frame generation' }).first()
    const frameGenValue = profile.fgOverride
      ? profile.fgEnabled
        ? 'true'
        : 'false'
      : '(default)'
    await expect(frameGenField.locator('.trigger')).toContainText(frameGenValue)
    await expect(page.getByLabel('HDR')).toBeChecked({ checked: profile.enableHdr })
    await expect(page.getByLabel('Wayland')).toBeChecked({ checked: profile.enableWayland })
  })

  test('can change quality mode value', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const qualityField = page.locator('.field', { hasText: 'Quality mode' })
    await qualityField.locator('.trigger').click()
    await qualityField.getByRole('button', { name: 'Performance', exact: true }).click()
    await expect(qualityField.locator('.trigger')).toContainText('Performance')
  })

  test('can toggle checkboxes', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const waylandCheckbox = page.getByLabel('Wayland')
    await expect(waylandCheckbox).not.toBeChecked()
    await waylandCheckbox.click()
    await expect(waylandCheckbox).toBeChecked()
  })

  test('shows save button', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByRole('button', { name: 'Save profile' })).toBeVisible()
  })

  test('save button shows saving state', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const saveButton = page.getByRole('button', { name: 'Save profile' })
    await saveButton.click()
    await expect(page.getByText('Profile saved!')).toBeVisible()
  })
})

test.describe('game without profile', () => {
  test('shows default profile values for new games', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Elden Ring').click()
    await expect(page.locator('h2', { hasText: 'DLSS settings' })).toBeVisible()
    const qualityField = page.locator('.field', { hasText: 'Quality mode' })
    await expect(qualityField.locator('.trigger')).toContainText('(default)')
  })
})
