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
