import { test, expect } from './fixtures.js'

test('default profile action submits one explicit atomic patch batch', async ({ page }) => {
  await page.goto('/')
  const hdrField = page.locator('.field', { has: page.getByLabel('HDR') })
  await hdrField.getByRole('button', { name: 'Unset' }).click()
  await page.getByRole('button', { name: 'Save default profile' }).click()
  await expect(page.getByText('Default profile saved!')).toBeVisible()
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
    await page.getByRole('button', { name: 'Save default profile' }).click()
    await expect(page.getByText('Default profile saved!')).toBeVisible()
    await expect.poll(() => page.evaluate(() => window.__profilePatchCalls.at(-1))).toEqual({
      scope: 'default', patches: [{ field: 'gpu.vrr', operation: 'set', value: policy }]
    })
  }
  await page.screenshot({ path: testInfo.outputPath('vrr-never-saved.png') })
  await field.getByRole('button', { name: 'Reset default', exact: true }).click()
  await page.getByRole('button', { name: 'Save default profile' }).click()
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
  await page.evaluate(() => {
    const app = window.go.gui.App
    const getProfile = app.GetProfile
    const patchProfile = app.PatchProfile
    let policy = 'always'
    let source = 'default'
    app.GetProfile = async (id) => ({
      ...await getProfile(id), clockOffset: 0, memoryOffset: 0, vrr: policy,
      semantics: [{ field: 'gpu.vrr', source, impact: 'system_state', restore: 'restorable_mutation' }]
    })
    app.PatchProfile = async (id, patches) => {
      await patchProfile(id, patches)
      const patch = patches.find(patch => patch.field === 'gpu.vrr')
      policy = patch.operation === 'reset' ? 'always' : patch.value
      source = patch.operation === 'reset' ? 'default' : 'override'
      return app.GetProfile(id)
    }
  })
  await page.getByText('Cyberpunk 2077', { exact: true }).click()
  await page.getByRole('tab', { name: '2 Profile', exact: true }).click()
  await page.getByRole('button', { name: 'GPU', exact: true }).click()
  const field = page.locator('.field', { has: page.locator('label', { hasText: 'KDE VRR' }) })
  await expect(field).toContainText('source default')
  await expect(field.locator('.trigger')).toContainText('Always')
  await field.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('vrr-game-inherited.png') })
  await field.locator('.trigger').click()
  await field.getByRole('button', { name: 'Unset — leave KDE unchanged', exact: true }).click()
  await page.getByRole('button', { name: 'Save profile', exact: true }).click()
  await expect(field).toContainText('source override')
  await expect(field.locator('.trigger')).toContainText('Unset')
  await page.screenshot({ path: testInfo.outputPath('vrr-game-unset.png') })
  await field.getByRole('button', { name: 'Reset', exact: true }).click()
  await page.getByRole('button', { name: 'Save profile', exact: true }).click()
  await expect(field).toContainText('source default')
  await expect(field.locator('.trigger')).toContainText('Always')
  await page.screenshot({ path: testInfo.outputPath('vrr-game-reset.png') })
})
