// One launched GitHub launch with alice queued, shared by the responsive and
// accessibility specs. Returns storage states so each test opens its own page.
import type { APIRequestContext, Browser } from '@playwright/test'
import { invitePath, launchGitHub, people, verifyAndConnect } from './app.ts'

export type World = {
  launchId: string
  alicePath: string // /s/<token>, verified and queued
  bobPath: string // /s/<token>, not yet verified
  captain: Awaited<ReturnType<import('@playwright/test').BrowserContext['storageState']>>
  alice: Awaited<ReturnType<import('@playwright/test').BrowserContext['storageState']>>
}

export async function buildWorld(browser: Browser, request: APIRequestContext, baseURL: string | undefined): Promise<World> {
  const cap = await browser.newContext({ baseURL })
  const page = await cap.newPage()
  const launchId = await launchGitHub(page, request)
  const alicePath = await invitePath(request, people.alice.pr)
  const bobPath = await invitePath(request, people.bob.pr)

  const al = await browser.newContext({ baseURL })
  const alice = await al.newPage()
  await verifyAndConnect(alice, request, alicePath, people.alice)
  await alice.getByRole('button', { name: 'Add to my queue' }).click()
  await alice.waitForURL(`**${alicePath}/done`)
  const world = { launchId, alicePath, bobPath, captain: await cap.storageState(), alice: await al.storageState() }
  await cap.close()
  await al.close()
  return world
}
