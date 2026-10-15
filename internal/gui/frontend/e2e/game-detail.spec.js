// The game page opens on its Profile, shows one subsystem at a time, and saves
// every control as it changes. The words come from the Go vocabulary through the
// profile projection. The DLL list moved to its own tab and is covered by
// GameDetail.test.js.
import { test, expect, games } from './fixtures.js'

const cyberpunk = games[0]

const patchCalls = (page) => page.evaluate(() => window.__profilePatchCalls)

test.describe('game detail', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
  })

  test('opens on the Profile tab with the game header', async ({ page }) => {
    await expect(page.locator('h1', { hasText: cyberpunk.name })).toBeVisible()
    await expect(page.getByRole('tab', { name: '1 Profile', exact: true })).toBeVisible()
    await expect(page.getByRole('tab', { name: '2 DLLs', exact: true })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Overview/ })).toHaveCount(0)
    const appIdRow = page.locator('.row', { hasText: 'App ID' })
    await expect(appIdRow.locator('.value')).toHaveText(String(cyberpunk.appId))
    await expect(page.getByText('Saved at once')).toBeVisible()
  })

  test('shows where the game lives on the DLLs tab only', async ({ page }) => {
    await expect(page.getByText(cyberpunk.installDir)).toHaveCount(0)
    await page.getByRole('tab', { name: '2 DLLs', exact: true }).click()
    await expect(page.getByText(cyberpunk.installDir)).toBeVisible()
    await expect(page.getByText(cyberpunk.prefixPath)).toBeVisible()
    await expect(page.getByText('spela %command%')).toBeVisible()
  })

  test('names what the game sets itself', async ({ page }) => {
    await expect(page.getByText(/^Own settings \(5\):/)).toBeVisible()
    await expect(page.getByText(/HDR On/).first()).toBeVisible()
  })
})

test.describe('profile settings save as they change', () => {
  test('has no Save button and saves a checkbox at once', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await expect(page.getByRole('button', { name: /^Save/ })).toHaveCount(0)
    const waylandCheckbox = page.getByLabel('Wayland')
    await expect(waylandCheckbox).not.toBeChecked()
    await waylandCheckbox.click()
    await expect(waylandCheckbox).toBeChecked()
    await expect(page.getByText('Saved: Wayland On')).toBeVisible()
    await expect(page.getByText('Saved at once')).toBeVisible()
    expect(await patchCalls(page)).toEqual([{
      scope: 'game', appId: 1091500, patches: [{ field: 'proton.enable_wayland', operation: 'set', value: true }]
    }])
  })

  test('Follow and Set for this game undo each other', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const clock = page.locator('.field', { has: page.getByLabel('Clock offset') })
    await page.getByRole('button', { name: 'GPU', exact: true }).click()
    await expect(page.getByLabel('Clock offset')).toHaveValue('100')
    await clock.getByRole('button', { name: 'Follow All games' }).click()
    await expect(page.getByLabel('Clock offset')).toHaveValue('0')
    await expect(page.getByText('Saved: Clock offset follows All games')).toBeVisible()
    await clock.getByRole('button', { name: 'Set for this game' }).click()
    await expect(page.getByLabel('Clock offset')).toHaveValue('100')
    expect((await patchCalls(page)).map(call => call.patches)).toEqual([
      [{ field: 'gpu.clock_offset', operation: 'reset' }],
      [{ field: 'gpu.clock_offset', operation: 'set', value: 100 }]
    ])
  })

  test('a failed save returns the control to the stored value and says so', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    const hdr = page.getByLabel('HDR')
    await expect(hdr).toBeChecked()
    await page.evaluate(() => { window.__failNextPatch = 'permission denied' })
    await hdr.click()
    await expect(page.getByText('Not saved: permission denied').first()).toBeVisible()
    await expect(hdr).toBeChecked()
    await page.getByRole('button', { name: 'Dismiss' }).click()
    await expect(page.getByRole('button', { name: 'Dismiss' })).toHaveCount(0)
  })

  test('number inputs save a whole number and refuse anything else', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await page.getByRole('button', { name: 'GPU', exact: true }).click()
    const fan = page.getByLabel('Fan speed')
    await fan.fill('fast')
    await fan.press('Enter')
    await expect(page.getByRole('alert')).toContainText('Enter a whole number')
    await expect(fan).toHaveValue('0')
    expect(await patchCalls(page)).toEqual([])
    await fan.fill('45')
    await fan.press('Enter')
    await expect(page.getByText('Saved: Fan speed 45%')).toBeVisible()
    expect((await patchCalls(page)).at(-1).patches).toEqual([{ field: 'gpu.fan_speed', operation: 'set', value: 45 }])
  })

  test('a failed number save shows the stored number again while the field has focus', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await page.getByRole('button', { name: 'GPU', exact: true }).click()
    const fan = page.getByLabel('Fan speed')
    await page.evaluate(() => { window.__failNextPatch = 'read-only' })
    await fan.fill('45')
    await fan.press('Enter')
    await expect(page.getByText('Not saved: read-only').first()).toBeVisible()
    await expect(fan).toBeFocused()
    await expect(fan).toHaveValue('0')
  })

  test('Follow All games for all settings confirms, then clears everything in one batch', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await page.getByRole('button', { name: 'Follow All games for all settings' }).click()
    const dialog = page.getByRole('dialog', { name: 'Follow All games for all settings' })
    await expect(dialog).toContainText('This clears the 5 settings')
    await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(dialog).toHaveCount(0)
    expect(await patchCalls(page)).toEqual([])

    await page.getByRole('button', { name: 'Follow All games for all settings' }).click()
    await dialog.getByRole('button', { name: 'Follow All games for all settings' }).click()
    await expect(page.getByText('Saved: All settings follow All games')).toBeVisible()
    expect(await patchCalls(page)).toHaveLength(1)
    expect((await patchCalls(page))[0].patches.map(patch => patch.operation)).toEqual(['reset', 'reset', 'reset', 'reset', 'reset'])
    await expect(page.getByText('Own settings: none, every value follows All games')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Follow All games for all settings' })).toBeDisabled()
  })

  test('the DLLs tab stays when the game changes', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Cyberpunk 2077').click()
    await page.getByRole('tab', { name: '2 DLLs', exact: true }).click()
    await page.getByText('The Witcher 3').click()
    await expect(page.locator('h1', { hasText: 'The Witcher 3' })).toBeVisible()
    await expect(page.getByText('/home/user/.steam/steam/steamapps/common/The Witcher 3')).toBeVisible()
  })
})

test.describe('game without profile', () => {
  test('says that every value follows All games', async ({ page }) => {
    await page.goto('/')
    await page.getByText('Elden Ring').click()
    await expect(page.getByText('This game has no profile of its own; every value follows All games.')).toBeVisible()
    await expect(page.getByText('from All games').first()).toBeVisible()
    await expect(page.getByText('Own settings: none, every value follows All games')).toBeVisible()
  })
})
