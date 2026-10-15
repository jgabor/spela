// The review's GUI tasks, counted as actions (a click, a typed entry, or a key
// press), at the review size and at a narrow window. Screenshots go to
// SPELA_E2E_SHOTS when it is set.
import { test, expect } from './fixtures.js'

const sizes = [{ name: '1280x800', width: 1280, height: 800 }, { name: 'narrow', width: 760, height: 800 }]

for (const size of sizes) {
  test.describe(`journeys at ${size.name}`, () => {
    test.use({ viewport: { width: size.width, height: size.height } })

    const shot = async (page, testInfo, name) => {
      const directory = process.env.SPELA_E2E_SHOTS
      await page.screenshot({ path: directory ? `${directory}/${size.name}-${name}.png` : testInfo.outputPath(`${name}.png`) })
    }
    const patches = (page) => page.evaluate(() => window.__profilePatchCalls)

    test('T1 near: Wayland on for the game under the cursor', async ({ page }, testInfo) => {
      let actions = 0
      await page.goto('/')
      await page.getByText('The Witcher 3').click(); actions++
      await shot(page, testInfo, 't1-opened-on-profile')
      await page.getByLabel('Wayland').click(); actions++
      await expect(page.getByText('Saved: Wayland On')).toBeVisible()
      await shot(page, testInfo, 't1-saved')
      expect((await patches(page)).at(-1).patches).toEqual([{ field: 'proton.enable_wayland', operation: 'set', value: true }])
      console.log(`${size.name} T1 near actions=${actions}`)
      expect(actions).toBeLessThanOrEqual(3)
    })

    test('T6: clock offset 200', async ({ page }, testInfo) => {
      let actions = 0
      await page.goto('/')
      await page.getByText('The Witcher 3').click(); actions++
      await page.getByRole('button', { name: 'GPU', exact: true }).click(); actions++
      const clock = page.getByLabel('Clock offset')
      await clock.fill('200'); actions++
      await shot(page, testInfo, 't6-typed')
      await clock.press('Enter'); actions++
      await expect(page.getByText('Saved: Clock offset 200 MHz')).toBeVisible()
      await shot(page, testInfo, 't6-saved')
      expect((await patches(page)).at(-1).patches).toEqual([{ field: 'gpu.clock_offset', operation: 'set', value: 200 }])
      console.log(`${size.name} T6 actions=${actions}`)
      expect(actions).toBeLessThanOrEqual(5)
    })

    test('T7: Wayland on for three games in a row', async ({ page }, testInfo) => {
      let actions = 0
      await page.goto('/')
      for (const name of ['Cyberpunk 2077', 'The Witcher 3', 'Elden Ring']) {
        await page.getByText(name).click(); actions++
        await page.getByLabel('Wayland').click(); actions++
        await expect(page.getByText('Saved: Wayland On')).toBeVisible()
      }
      await shot(page, testInfo, 't7-third-game')
      const calls = await patches(page)
      expect(calls.map(call => call.appId)).toEqual([1091500, 292030, 1245620])
      console.log(`${size.name} T7 actions=${actions}`)
      expect(actions).toBeLessThanOrEqual(7)
    })

    test('T5: one field follows, then the whole game', async ({ page }, testInfo) => {
      let actions = 0
      await page.goto('/')
      await page.getByText('Cyberpunk 2077').click(); actions++
      await page.getByRole('button', { name: 'GPU', exact: true }).click(); actions++
      await page.locator('.field', { has: page.getByLabel('Clock offset') }).getByRole('button', { name: 'Follow All games' }).click(); actions++
      await expect(page.getByText('Saved: Clock offset follows All games')).toBeVisible()
      await page.getByRole('button', { name: 'Follow All games for all settings' }).click(); actions++
      await shot(page, testInfo, 't5-confirm')
      await page.getByRole('dialog').getByRole('button', { name: 'Follow All games for all settings' }).click(); actions++
      await expect(page.getByText('Own settings: none, every value follows All games')).toBeVisible()
      await shot(page, testInfo, 't5-saved')
      console.log(`${size.name} T5 actions=${actions}`)
      expect(actions).toBeLessThanOrEqual(9)
    })
  })
}
