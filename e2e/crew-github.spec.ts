// Spec 7, end to end: GitHub crew. Open the PR comment link, verify GitHub,
// connect Buffer, tap; the post is in the Buffer fake with a jittered dueAt
// inside the launch window.
import { expect, test } from '@playwright/test'
import { control, expectInWindow, invitePath, launchGitHub, newPerson, people, verifyAndConnect } from './fixtures/app.ts'

test('GitHub crew post lands in the window', async ({ page, request, browser, baseURL }) => {
  await control.reset(request)
  const launchId = await launchGitHub(page, request)
  const path = await invitePath(request, people.alice.pr)

  const { ctx, page: alice } = await newPerson(browser, baseURL)
  await alice.goto(path)
  await expect(alice.getByText('Verify it\'s you to see your card').or(alice.getByRole('link', { name: "Verify it's you with GitHub" }))).toBeVisible()
  await verifyAndConnect(alice, request, path, people.alice)
  await expect(alice.getByRole('radio', { name: 'Land in the window' })).toBeChecked()
  await alice.getByRole('button', { name: 'Add to my queue' }).click()
  await expect(alice).toHaveURL(new RegExp(`${path}/done$`))
  await expect(alice.getByRole('heading', { level: 1 })).toHaveText(/^Queued for /)

  const launch = (await control.launches(request)).find((l) => l.id === launchId)!
  const mine = (await control.posts(request)).filter((p) => p.channelId.startsWith('alice-'))
  expect(mine.length).toBeGreaterThan(0)
  for (const p of mine) {
    expect(p.mode).toBe('customScheduled')
    expectInWindow(p.dueAt, launch)
  }
  await ctx.close()
})
