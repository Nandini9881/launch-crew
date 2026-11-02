// Spec 7, end to end: judges' path. The launch window has closed; the
// invite link still queues, via addToQueue.
import { expect, test } from '@playwright/test'
import { control, invitePath, launchGitHub, newPerson, people, until, verifyAndConnect } from './fixtures/app.ts'

test('closed launch still queues', async ({ page, request, browser, baseURL }) => {
  await control.reset(request)
  const id = await launchGitHub(page, request)
  const path = await invitePath(request, people.alice.pr)

  // The real CloseWindow job, pulled forward to now.
  await until(async () => (await control.closeWindow(request, id)).rows === 1, 'close_window job')
  await until(async () => (await control.launches(request)).find((l) => l.id === id)?.status === 'closed', 'launch closed')

  const { ctx, page: alice } = await newPerson(browser, baseURL)
  await verifyAndConnect(alice, request, path, people.alice)
  await expect(alice.getByRole('radio', { name: 'Land in the window' })).toHaveCount(0)
  await expect(alice.getByRole('radio', { name: 'next free slot' })).toBeChecked()
  await alice.getByRole('button', { name: 'Add to my queue' }).click()
  await expect(alice).toHaveURL(new RegExp(`${path}/done$`))

  const mine = (await control.posts(request)).filter((p) => p.channelId.startsWith('alice-'))
  expect(mine.length).toBeGreaterThan(0)
  for (const p of mine) {
    expect(p.mode).toBe('addToQueue')
    expect(p.dueAt ?? '').toBe('')
    expect(p.saveToDraft).toBe(false)
  }
  await ctx.close()
})
