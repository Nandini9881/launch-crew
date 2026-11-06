import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import App from '../App'
import type { BoardView, DraftView, Outcome, View } from '../api'

const at = (h: number, m = 0) => new Date(2026, 10, 3, h, m) // Tue 3 Nov 2026, local
const invite = (launchAt: Date | null = at(9), shortUrl = '', releaseUrl = 'https://x/r') =>
  ({
    launch: { id: 'l1', title: 'Ship', project: 'acme', version: 'v1', releaseUrl, shortUrl, launchAt: launchAt?.toISOString() ?? null, windowMinutes: 20 },
    member: { id: 'm1', name: 'Ada' },
    card: null, preview: '', verified: true, foreign: false, needsVerify: false, copyText: '',
  }) as unknown as View

const out = (o: Partial<Outcome>): Outcome => ({ channelId: 'c1', service: 'twitter', status: 'queued', when: at(9).toISOString(), note: '', postId: 'p1', ...o })
const row = (name: string, state: string) => ({ name, role: 'Speaker', state, when: null })
const crewOf = (rows: ReturnType<typeof row>[], inN: number): BoardView => ({ rows, in: inN, total: rows.length })

type Opts = { board?: BoardView | 'fail'; launchAt?: Date | null; drafts?: Partial<DraftView>[]; remember?: number; shortUrl?: string; releaseUrl?: string }
function mockApi(o: Opts = {}) {
  const calls: { method: string; url: string; body?: string }[] = []
  const json = (status: number, body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    calls.push({ method, url, body: init?.body as string | undefined })
    if (url === '/api/s/tok1') return json(200, invite(o.launchAt, o.shortUrl, o.releaseUrl))
    if (url.endsWith('/board')) return o.board === 'fail' ? json(500, { error: 'internal' }) : json(200, o.board ?? crewOf([row('Ada', 'queued')], 1))
    if (url.endsWith('/drafts')) return json(200, { drafts: o.drafts ?? [], voicedReady: true })
    if (url.endsWith('/remember')) return o.remember ? json(o.remember, { error: 'x' }) : Promise.resolve(new Response(null, { status: 204 }))
    return json(404, { error: 'not_found' })
  }))
  return calls
}

function visit(outcomes?: Outcome[]) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: '/s/tok1/done', state: outcomes ? { outcomes } : undefined }]}>
      <App />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

test('crew of one empty state', async () => {
  mockApi({ board: crewOf([row('Ada', 'queued')], 1) })
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
  visit([out({})])
  expect(await screen.findByText('Just you so far. Share the link and the board fills in.')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Copy link' }))
  expect(writeText).toHaveBeenCalledWith('https://x/r')
})

test('board failure keeps queued line', async () => {
  const calls = mockApi({ board: 'fail' })
  visit([out({})])
  expect(await screen.findByRole('heading', { level: 1, name: 'Queued for Tue 9:00 AM' })).toBeInTheDocument()
  expect(await screen.findByText(/crew board is taking a moment/)).toBeInTheDocument()
  const retry = screen.getByRole('button', { name: 'Retry' })
  await userEvent.click(retry)
  expect(calls.filter((c) => c.url.endsWith('/board'))).toHaveLength(2)
  expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Queued for Tue 9:00 AM')
})

test('mixed states render', async () => {
  mockApi({
    board: crewOf([row('Ada', 'queued'), row('Bo', 'posted'), row('Cy', 'copied'), row('Di', 'invited'), row('Ed', 'opened'), row('Flo', 'comment_failed')], 3),
  })
  visit([out({}), out({ channelId: 'c2', service: 'bluesky', when: at(9, 12).toISOString(), postId: 'p2' })])
  expect(await screen.findByRole('heading', { name: 'Crew · 3 of 6 in' })).toBeInTheDocument()
  const item = (n: string) => screen.getByText(n).closest('li')!
  expect(within(item('Ada')).getByText('queued')).toHaveClass('row-state')
  expect(item('Ada')).toHaveClass('row-ok')
  expect(item('Bo')).toHaveClass('row-ok')
  expect(item('Cy')).toHaveClass('row-muted')
  expect(within(item('Cy')).getByText('shared another way')).toBeInTheDocument()
  expect(item('Di')).toHaveClass('row-muted')
  expect(item('Ed')).toHaveClass('row-muted')
  expect(item('Flo')).toHaveClass('row-err')
  expect(screen.queryByText(/Just you so far/)).toBeNull()
  // one line per outcome
  expect(screen.getByText('X')).toBeInTheDocument()
  expect(screen.getByText('Bluesky')).toBeInTheDocument()
  expect(screen.getByText('Posts land between 9:00 and 9:20 so the timeline fills in instead of a wall.')).toBeInTheDocument()
})

test('headline cases, open link and caption', async () => {
  mockApi()
  const first = visit([out({ when: at(10).toISOString() }), out({ channelId: 'c2', when: at(9, 30).toISOString(), postId: 'p2' })])
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Queued for Tue 9:30 AM')
  const link = screen.getByRole('link', { name: 'Open in Buffer' })
  expect(link).toHaveAttribute('href', 'https://publish.buffer.com/posts/p1')
  expect(link).toHaveAttribute('target', '_blank')
  expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  first.unmount()

  mockApi()
  const d = visit([out({ status: 'draft', when: null, postId: '' })])
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Saved as a draft')
  expect(screen.getByRole('link', { name: 'Open in Buffer' })).toHaveAttribute('href', 'https://publish.buffer.com')
  expect(screen.queryByText(/Posts land between/)).toBeNull()
  d.unmount()

  mockApi()
  visit([out({ when: null })])
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Added to your queue')
})

test('no caption without a launch time', async () => {
  mockApi({ launchAt: null })
  visit([out({})])
  await screen.findByRole('heading', { name: /^Crew/ })
  expect(screen.queryByText(/Posts land between/)).toBeNull()
})

test('reload falls back to drafts', async () => {
  mockApi({ drafts: [{ id: 'd1', channelId: 'c1', network: 'twitter', status: 'queued', bufferPostId: 'p7' }] })
  const a = visit()
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Added to your queue')
  expect(screen.getByRole('link', { name: 'Open in Buffer' })).toHaveAttribute('href', 'https://publish.buffer.com/posts/p7')
  a.unmount()

  mockApi({ drafts: [{ id: 'd1', channelId: 'c1', network: 'twitter', status: 'draft', bufferPostId: 'p8' }] })
  const b = visit()
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Saved as a draft')
  expect(screen.getByRole('link', { name: 'Open in Buffer' })).toHaveAttribute('href', 'https://publish.buffer.com/posts/p8')
  b.unmount()

  mockApi({ drafts: [{ id: 'd1', channelId: 'c1', network: 'twitter', status: 'draft', bufferPostId: '' }] })
  const c = visit()
  expect(await screen.findByRole('heading', { level: 1, name: 'Nothing queued yet' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /approve|back/i })).toHaveAttribute('href', '/s/tok1/approve')
  c.unmount()

  mockApi({ drafts: [] })
  visit()
  expect(await screen.findByRole('heading', { level: 1, name: 'Nothing queued yet' })).toBeInTheDocument()
})

test('caption shows both meridiems across noon', async () => {
  mockApi({ launchAt: at(11, 50) })
  visit([out({})])
  expect(await screen.findByText('Posts land between 11:50 AM and 12:10 PM so the timeline fills in instead of a wall.')).toBeInTheDocument()
})

test('board refreshes once after 30 seconds', async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  const calls = mockApi()
  visit([out({})])
  await screen.findByRole('heading', { name: /^Crew/ })
  expect(calls.filter((c) => c.url.endsWith('/board'))).toHaveLength(1)
  await act(async () => { await vi.advanceTimersByTimeAsync(31000) })
  expect(calls.filter((c) => c.url.endsWith('/board'))).toHaveLength(2)
  await act(async () => { await vi.advanceTimersByTimeAsync(120000) })
  expect(calls.filter((c) => c.url.endsWith('/board'))).toHaveLength(2)
})

test('remember me saves and reverts on failure', async () => {
  const calls = mockApi()
  visit([out({})])
  const box = await screen.findByRole('checkbox', { name: 'Remember me for future launches' })
  await userEvent.click(box)
  expect(box).toBeChecked()
  const sent = calls.find((c) => c.url === '/api/crew/m1/remember')!
  expect(JSON.parse(sent.body!)).toEqual({ remember: true })
  expect(sent.method).toBe('POST')
})

test('remember me reverts silently on failure', async () => {
  mockApi({ remember: 500 })
  visit([out({})])
  const box = await screen.findByRole('checkbox', { name: 'Remember me for future launches' })
  await userEvent.click(box)
  expect(await screen.findByText("couldn't save, try again")).toBeInTheDocument()
  expect(box).not.toBeChecked()
})

test('manual launch copies the short link', async () => {
  mockApi({ board: crewOf([row('Speaker', 'queued')], 1), shortUrl: 'https://lc.test/l/abc', releaseUrl: '' })
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
  visit([out({})])
  await userEvent.click(await screen.findByRole('button', { name: 'Copy link' }))
  expect(writeText).toHaveBeenCalledWith('https://lc.test/l/abc')
  expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
})

test('no link to copy, no copy button', async () => {
  mockApi({ board: crewOf([row('Speaker', 'queued')], 1), releaseUrl: '' })
  visit([out({})])
  expect(await screen.findByText('Just you so far. Share the link and the board fills in.')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /Copy link|Copied/ })).toBeNull()
})

test('board shows the role only when it differs from the name', async () => {
  mockApi({ board: crewOf([row('Speaker', 'queued'), { name: 'ada', role: 'contributor', state: 'invited', when: null }], 1) })
  visit([out({})])
  const speaker = (await screen.findByText('Speaker')).closest('li')!
  expect(within(speaker).queryAllByText('Speaker')).toHaveLength(1)
  const ada = screen.getByText('ada').closest('li')!
  expect(within(ada).getByText('contributor')).toBeInTheDocument()
})

test('a failed drafts fetch on reload never reads as empty; Retry asks again', async () => {
  const calls = mockApi({ drafts: [{ id: 'd1', channelId: 'c1', network: 'twitter', status: 'queued', bufferPostId: 'p7' }] })
  const base = (globalThis.fetch as ReturnType<typeof vi.fn>).getMockImplementation()!
  let fail = true
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    if (url.endsWith('/drafts') && fail) {
      fail = false
      calls.push({ method: 'GET', url })
      return Promise.resolve(new Response(JSON.stringify({ error: 'internal' }), { status: 500 }))
    }
    return base(url, init)
  }))
  visit()
  const alert = await screen.findByRole('alert')
  expect(alert).toHaveTextContent(/could not load what was queued/i)
  expect(screen.queryByRole('heading', { name: 'Nothing queued yet' })).toBeNull()
  await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }))
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Added to your queue')
  expect(calls.filter((c) => c.url.endsWith('/drafts'))).toHaveLength(2)
})
