import { test, expect } from './fixtures.js'
import { fieldSemantics } from '../src/lib/testSemantics.js'

test('default profile action submits one explicit atomic patch batch', async ({ page }) => {
  await page.goto('/')
  const hdrField = page.locator('.field', { has: page.getByLabel('HDR') })
  await hdrField.getByRole('button', { name: 'Use system default' }).click()
  await expect(page.getByText(/^Saved: HDR/)).toBeVisible()
  await expect.poll(() => page.evaluate(() => window.__profilePatchCalls)).toEqual([{
    scope: 'default',
    patches: [{ field: 'proton.enable_hdr', operation: 'reset' }]
  }])
})

test('KDE VRR choices save literal policies and reset separately', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1200, height: 900 })
  await page.goto('/')
  await page.getByRole('button', { name: 'GPU', exact: true }).click()
  const field = page.locator('.field', { has: page.locator('label', { hasText: 'KDE VRR' }) })
  await expect(field.locator('.trigger')).toContainText('Unconfigured')
  await field.locator('.trigger').click()
  await expect(field.locator('.option')).toHaveCount(4)
  await page.screenshot({ path: testInfo.outputPath('vrr-choices.png') })
  await field.getByRole('button', { name: 'Unset — leave KDE unchanged', exact: true }).click()
  for (const [policy, label] of [['unset', 'Unset'], ['automatic', 'Automatic'], ['always', 'Always'], ['never', 'Never']]) {
    if (policy !== 'unset') {
      await field.locator('.trigger').click()
      await field.locator('.option').filter({ hasText: label }).click()
    }
    await expect(page.getByText('Saved at once')).toBeVisible()
    await expect.poll(() => page.evaluate(() => window.__profilePatchCalls.at(-1))).toEqual({
      scope: 'default', patches: [{ field: 'gpu.vrr', operation: 'set', value: policy }]
    })
  }
  await page.screenshot({ path: testInfo.outputPath('vrr-never-saved.png') })
  await field.getByRole('button', { name: 'Use system default', exact: true }).click()
  await expect(field.locator('.trigger')).toContainText('Unconfigured')
  await expect.poll(() => page.evaluate(() => window.__profilePatchCalls.at(-1))).toEqual({
    scope: 'default', patches: [{ field: 'gpu.vrr', operation: 'reset' }]
  })
  await page.screenshot({ path: testInfo.outputPath('vrr-reset.png') })
})

test('KDE VRR game editor distinguishes inherited policy and unset override', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1200, height: 900 })
  await page.goto('/')
  // Supply resolved views as the real backend does; Go boundary tests verify
  // the corresponding YAML persistence and defaults resolution.
  await page.evaluate((words) => {
    const app = window.go.gui.App
    const getProfile = app.GetProfile
    const patchProfile = app.PatchProfile
    let policy = 'always'
    let source = 'default'
    app.GetProfile = async (id) => ({
      ...await getProfile(id), clockOffset: 0, memoryOffset: 0, vrr: policy,
      semantics: words[source]
    })
    app.PatchProfile = async (id, patches) => {
      await patchProfile(id, patches)
      const patch = patches.find(patch => patch.field === 'gpu.vrr')
      policy = patch.operation === 'reset' ? 'always' : patch.value
      source = patch.operation === 'reset' ? 'default' : 'override'
      return app.GetProfile(id)
    }
  }, { default: fieldSemantics(), override: fieldSemantics({ 'gpu.vrr': 'override' }) })
  await page.getByText('Cyberpunk 2077', { exact: true }).click()
  await page.getByRole('button', { name: 'GPU', exact: true }).click()
  const field = page.locator('.field', { has: page.locator('label', { hasText: 'KDE VRR' }) })
  await expect(field).toContainText('from All games')
  await expect(field.locator('.trigger')).toContainText('Always')
  await field.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('vrr-game-inherited.png') })
  await field.locator('.trigger').click()
  await field.getByRole('button', { name: 'Unset — leave KDE unchanged', exact: true }).click()
  await expect(field).toContainText('this game')
  await expect(field.locator('.trigger')).toContainText('Unset')
  await page.screenshot({ path: testInfo.outputPath('vrr-game-unset.png') })
  await field.getByRole('button', { name: 'Follow All games', exact: true }).click()
  await expect(field).toContainText('from All games')
  await expect(field.locator('.trigger')).toContainText('Always')
  await page.screenshot({ path: testInfo.outputPath('vrr-game-reset.png') })
})

test('All games: Use system default and Set here undo each other, and a value at the system default only offers Set here', async ({ page }) => {
  await page.goto('/')
  const hdr = page.locator('.field', { has: page.getByLabel('HDR') })
  await hdr.getByRole('button', { name: 'Use system default' }).click()
  await expect(page.getByText('Saved: HDR uses the system default (All games)')).toBeVisible()
  await expect(hdr.getByRole('button', { name: 'Use system default' })).toHaveCount(0)
  await hdr.getByRole('button', { name: 'Set here' }).click()
  await expect.poll(() => page.evaluate(() => window.__profilePatchCalls.map((call) => call.patches))).toEqual([
    [{ field: 'proton.enable_hdr', operation: 'reset' }],
    [{ field: 'proton.enable_hdr', operation: 'set', value: false }]
  ])
  // A value that is not set there has no Use system default to press.
  const wayland = page.locator('.field', { has: page.getByLabel('Wayland') })
  await expect(wayland.getByRole('button', { name: 'Use system default' })).toHaveCount(0)
  await expect(wayland.getByRole('button', { name: 'Set here' })).toBeVisible()
})

test('All games: Apply to games with their own value asks, then those games follow and nothing else changes', async ({ page }) => {
  await page.goto('/')
  // A second game sets HDR itself, so two games have their own value.
  await page.getByText('The Witcher 3').click()
  await page.getByLabel('HDR').click()
  await expect(page.getByText('Saved: HDR On')).toBeVisible()
  await page.getByText('All games', { exact: true }).first().click()
  const hdr = page.locator('.field', { has: page.getByLabel('HDR') })
  await expect(hdr).toContainText('set here · 2 games have their own value')
  await expect(page.getByText(/does not reach|Applied when the game starts/)).toHaveCount(0)

  await hdr.getByRole('button', { name: 'Apply to games with their own value' }).click()
  const dialog = page.getByRole('dialog', { name: 'Apply to games with their own value' })
  await expect(dialog).toContainText('HDR: 2 games have their own value: Cyberpunk 2077, The Witcher 3: Wild Hunt. They will follow All games (Off).')
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  expect(await page.evaluate(() => window.__applyCalls)).toEqual([])

  await hdr.getByRole('button', { name: 'Apply to games with their own value' }).click()
  await dialog.getByRole('button', { name: 'Apply to games with their own value' }).click()
  await expect(page.getByText('Saved: HDR: 2 games follow All games')).toBeVisible()
  expect(await page.evaluate(() => window.__applyCalls)).toEqual([['proton.enable_hdr']])
  await expect(hdr.getByRole('button', { name: 'Apply to games with their own value' })).toHaveCount(0)
  await expect(hdr).not.toContainText('have their own value')

  // The game that kept another value of its own still has it.
  await page.getByText('Cyberpunk 2077').click()
  await expect(page.locator('.field', { has: page.getByLabel('HDR') })).toContainText('from All games')
  await page.getByRole('button', { name: 'GPU', exact: true }).click()
  await expect(page.locator('.field', { has: page.getByLabel('Clock offset') })).toContainText('this game')
})
