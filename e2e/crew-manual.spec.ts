// Spec 7, end to end: manual crew. Open the launch link, pick a role,
// connect, tap; the post exists and the captain sees the role.
import { expect, test } from '@playwright/test'
import { control, launchButton, localInput, newPerson, signInCaptain } from './fixtures/app.ts'

test('manual crew picks a role and posts', async ({ page, request, browser, baseURL }) => {
  await control.reset(request)
  await signInCaptain(page, request)
  await page.goto('/captain/new')
  await page.getByLabel('Title').fill('Rocket meetup')
  await page.getByLabel('Launch time').fill(localInput(60))
  await page.getByRole('button', { name: 'Create launch' }).click()
  await expect(page).toHaveURL(/\/captain\/launches\/[0-9a-f-]+$/)
  await expect(launchButton(page)).toHaveText('Launch')
  await launchButton(page).click()
  await expect(launchButton(page)).toHaveText('Launched')
  const openUrl = await page.getByLabel('Add by link').inputValue()
  expect(openUrl).toMatch(/\/l\/[A-Za-z0-9_-]+$/)

  const { ctx, page: dana } = await newPerson(browser, baseURL)
  await control.bufferAccount(request, 'dana')
  await dana.goto(new URL(openUrl).pathname)
  await dana.getByRole('radio', { name: /Speaker/ }).check()
  await dana.getByLabel('Talk title').fill('Shipping rockets')
  await dana.getByRole('button', { name: 'Continue' }).click()
  await expect(dana).toHaveURL(/\/s\/[A-Za-z0-9_-]+$/)
  await dana.getByRole('link', { name: 'Connect Buffer & share' }).click()
  await expect(dana).toHaveURL(/\/approve$/)
  await expect(dana.getByRole('textbox', { name: 'Your post' }).first()).toBeVisible()
  await dana.getByRole('button', { name: 'Add to my queue' }).click()
  await expect(dana).toHaveURL(/\/done$/)

  const mine = (await control.posts(request)).filter((p) => p.channelId.startsWith('dana-'))
  expect(mine.length).toBeGreaterThan(0)
  await page.reload()
  await expect(page.locator('.cap-crew-row').filter({ hasText: 'Speaker' })).toBeVisible()
  await ctx.close()
})
