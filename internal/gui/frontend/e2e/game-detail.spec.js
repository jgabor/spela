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
    await expect(page.locator('h2', { hasText: 'Detected DLLs' })).toBeVisible()
  })

  test('displays DLL names and versions', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByText('nvngx_dlss.dll')).toBeVisible()
    await expect(page.getByText('nvngx_dlssg.dll')).toBeVisible()
  })

  test('shows update badge for DLLs with updates', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByText('→ 3.8.0')).toBeVisible()
  })

  test('does not show DLL section for games without DLLs', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Elden Ring').click()
    await expect(page.locator('h2', { hasText: 'Detected DLLs' })).not.toBeVisible()
  })
})

test.describe('profile settings', () => {
  test('shows profile settings heading', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.locator('h2', { hasText: 'Profile settings' })).toBeVisible()
  })

  test('displays preset dropdown', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const presetSelect = page.locator('#preset')
    await expect(presetSelect).toBeVisible()
    await expect(presetSelect).toHaveValue(profiles[1091500].preset)
  })

  test('displays DLSS-SR mode dropdown', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const srModeSelect = page.locator('#srMode')
    await expect(srModeSelect).toBeVisible()
    await expect(srModeSelect).toHaveValue(profiles[1091500].srMode)
  })

  test('displays checkbox options', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByLabel('DLSS-SR override')).toBeVisible()
    await expect(page.getByLabel('Frame generation')).toBeVisible()
    await expect(page.getByLabel('HDR')).toBeVisible()
    await expect(page.getByLabel('Wayland')).toBeVisible()
    await expect(page.getByLabel('NGX Updater')).toBeVisible()
  })

  test('checkbox values match profile', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const profile = profiles[1091500]
    await expect(page.getByLabel('Frame generation')).toBeChecked({ checked: profile.fgEnabled })
    await expect(page.getByLabel('HDR')).toBeChecked({ checked: profile.enableHdr })
    await expect(page.getByLabel('Wayland')).toBeChecked({ checked: profile.enableWayland })
  })

  test('can change preset value', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const presetSelect = page.locator('#preset')
    await presetSelect.selectOption('performance')
    await expect(presetSelect).toHaveValue('performance')
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
    await expect(page.locator('h2', { hasText: 'Profile settings' })).toBeVisible()
    const presetSelect = page.locator('#preset')
    await expect(presetSelect).toHaveValue('balanced')
  })
})
