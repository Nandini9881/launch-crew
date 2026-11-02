import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import App from '../App'
import type { Check, Detail, LaunchSummary } from '../api'

const checks = (red: number): Check[] =>
  Array.from({ length: 7 }, (_, i) => ({
    id: i + 1,
    name: `Check ${i + 1}`,
    ok: i >= red,
    fix: i < red ? `Fix ${i + 1}` : '',
  }))

const detail = (over: Partial<Detail> = {}): Detail => ({
  id: 'l1',
  status: 'draft',
  source: 'github',
  title: 'rocket v1.2.0',
  summary: 's',
  releaseTag: 'v1.2.0',
  releaseUrl: 'https://github.com/acme/rocket/releases/tag/v1.2.0',
  launchAt: null,
  windowMinutes: 90,
  crewCap: 200,
  openCount: 0,
  openUrl: '',
  showUrl: '',
  pngUrl: '',
  roles: [],
  bufferConnected: true,
  headlines: [{ prNumber: 11, title: 'Faster boosters', authorLogin: 'alice', hidden: false }],
  chores: [],
  captainDrafts: [],
  crew: [],
  checks: checks(0),
  deadLetters: [],
  counts: { in: 0, total: 0 },
  ...over,
})

type Call = { method: string; url: string; body?: unknown }

function mockApi(routes: { detail?: Detail; list?: LaunchSummary[] } = {}) {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET'
      calls.push({ method, url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      const ok = (b: unknown, s = 200) => Promise.resolve(new Response(JSON.stringify(b), { status: s }))
      if (url === '/api/captain/launches' && method === 'GET') return ok(routes.list ?? [])
      if (url.includes('/retry/')) return ok({ status: 'retrying' }, 202)
      return ok(routes.detail ?? detail())
    }),
  )
  return calls
}

function visit(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

afterEach(() => vi.unstubAllGlobals())

test('launch disabled label counts red checks', async () => {
  mockApi({ detail: detail({ checks: checks(2) }) })
  visit('/captain/launches/l1')
  const btn = await screen.findByRole('button', { name: 'Launch · 2 checks left' })
  expect(btn).toBeDisabled()
})

test('launch enabled when all checks are green', async () => {
  mockApi()
  visit('/captain/launches/l1')
  expect(await screen.findByRole('button', { name: 'Launch' })).toBeEnabled()
})

test('dead letter row first with retry', async () => {
  const calls = mockApi({
    detail: detail({ deadLetters: [{ jobId: 42, kind: 'launch', lastError: 'preflight not green', attempts: 5 }] }),
  })
  visit('/captain/launches/l1')
  const alert = await screen.findByRole('alert')
  expect(alert).toHaveTextContent('preflight not green')
  const h1 = screen.getByRole('heading', { level: 1 })
  expect(alert.compareDocumentPosition(h1) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  const main = screen.getByRole('main')
  expect(within(main).getAllByRole('button')[0]).toBe(within(alert).getByRole('button', { name: 'Retry' }))
  await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }))
  expect(calls.some((c) => c.method === 'POST' && c.url === '/api/captain/launches/l1/retry/42')).toBe(true)
})

test('red preflight row shows fix text', async () => {
  mockApi({ detail: detail({ checks: checks(1) }) })
  visit('/captain/launches/l1')
  const section = await screen.findByRole('region', { name: 'Preflight' })
  const list = within(section).getByRole('list')
  expect(list.tagName).toBe('OL')
  expect(within(list).getAllByRole('listitem')).toHaveLength(7)
  expect(within(section).getByText('Fix 1')).toBeInTheDocument()
  expect(within(section).queryByText('Fix 2')).toBeNull()
})

test('empty launches state', async () => {
  mockApi({ list: [] })
  visit('/captain')
  expect(await screen.findByText('Nothing launched yet. Tag a release, or start one by hand.')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Connect a repo' })).toHaveAttribute('href', '/captain/repos')
  expect(screen.getByRole('link', { name: 'Start one by hand' })).toHaveAttribute('href', '/captain/new')
})

test('new launch roles capped at four and 24 chars', async () => {
  mockApi()
  visit('/captain/new')
  const add = await screen.findByRole('button', { name: 'Add a role' })
  for (let i = 0; i < 6; i++) if (!(add as HTMLButtonElement).disabled) await userEvent.click(add)
  const names = screen.getAllByLabelText(/^Role \d name$/)
  expect(names).toHaveLength(4)
  names.forEach((n) => expect(n).toHaveAttribute('maxlength', '24'))
  expect(add).toBeDisabled()
})

test('hero url must be https', async () => {
  const calls = mockApi()
  visit('/captain/new')
  await userEvent.type(await screen.findByLabelText('Title'), 'Go night')
  await userEvent.type(screen.getByLabelText('Hero image URL (optional)'), 'http://x.test/a.png')
  await userEvent.click(screen.getByRole('button', { name: 'Create launch' }))
  expect(await screen.findByText('Use an https:// image URL.')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)
})

test('crew state words drop underscores', async () => {
  const row = { id: 'm1', name: 'carol', role: 'contributor', state: 'opted_out', commentFailed: false, inviteUrl: '' }
  mockApi({ detail: detail({ crew: [row as unknown as Detail['crew'][number]], counts: { in: 0, total: 1 } }) })
  visit('/captain/launches/l1')
  expect(await screen.findByText('opted out')).toBeInTheDocument()
})

const post = (over: Partial<Detail['captainDrafts'][number]>): Detail['captainDrafts'][number] => ({
  id: 'p1', channelId: 'c1', channelName: 'Acme X', service: 'twitter', text: 'v1.2', status: 'queued', bufferPostId: 'b1', note: '', ...over,
})

test('captain posts show draft and failed notes', async () => {
  mockApi({
    detail: detail({
      status: 'launched',
      captainDrafts: [
        post({ id: 'a', channelName: 'Acme X', status: 'draft', note: 'your queue is full, saved as a draft instead' }),
        post({ id: 'b', channelName: 'Acme LinkedIn', service: 'linkedin', status: 'failed', bufferPostId: '', note: 'Buffer refused it: Text too long' }),
      ],
    }),
  })
  visit('/captain/launches/l1')
  const posts = await screen.findByRole('region', { name: 'Your posts' })
  expect(within(posts).getByText('your queue is full, saved as a draft instead')).toBeInTheDocument()
  const failed = within(posts).getByText('Buffer refused it: Text too long')
  expect(failed.closest('li')).toHaveTextContent('failed')
})

test('manual draft promises links after launch', async () => {
  mockApi({ detail: detail({ source: 'manual', headlines: [] }) })
  visit('/captain/launches/l1')
  expect(await screen.findByText('Link, slide and PNG appear after Launch.')).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'Show at the event' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Copy link' })).toBeNull()
})

test('launched manual launch shows link, slide and PNG', async () => {
  mockApi({
    detail: detail({ source: 'manual', status: 'launched', openUrl: 'https://lc.test/l/t', showUrl: 'https://lc.test/l/t/show', pngUrl: 'https://lc.test/api/l/t/show.png' }),
  })
  visit('/captain/launches/l1')
  expect(await screen.findByRole('link', { name: 'Show at the event' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
  expect(screen.queryByText('Link, slide and PNG appear after Launch.')).toBeNull()
})
