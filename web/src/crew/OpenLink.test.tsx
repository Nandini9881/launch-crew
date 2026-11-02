import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import App from '../App'
import type { LaunchView } from '../api'

const launch: LaunchView = {
  title: 'GoMeetup Nov',
  summary: 's',
  roles: [
    { name: 'Organizer', hint: 'ran the night' },
    { name: 'Speaker', hint: 'gave a talk' },
    { name: 'Attendee', hint: 'came along' },
  ],
  card: { url: '', alt: '' },
  shortUrl: 'https://lc.test/l/open1',
  openCount: 1,
}

function reply(status: number, body: unknown) {
  return Promise.resolve(new Response(JSON.stringify(body), { status }))
}

type Posted = { url: string; body: Record<string, unknown> }

function mockApi(opts: { get?: [number, unknown]; post?: [number, unknown] } = {}) {
  const posts: Posted[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (url.startsWith('/api/s/')) return reply(404, { error: 'not_found', message: 'm' })
      if (init?.method === 'POST') {
        posts.push({ url, body: JSON.parse(String(init.body)) })
        const [s, b] = opts.post ?? [200, { token: 'tok 9', memberId: 'm9' }]
        return reply(s, b)
      }
      const [s, b] = opts.get ?? [200, launch]
      return reply(s, b)
    }),
  )
  return posts
}

function visit(path = '/l/open1') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

afterEach(() => vi.unstubAllGlobals())

test('attendee preselected, continue posts role', async () => {
  const posts = mockApi()
  visit()
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('GoMeetup Nov')
  expect(screen.getByRole('group', { name: 'What was your part?' })).toBeInTheDocument()
  expect(screen.getByRole('radio', { name: /Attendee/ })).toBeChecked()
  expect(screen.getAllByRole('radio').map((r) => r.closest('label')?.textContent)).toEqual([
    'Organizerran the night',
    'Speakergave a talk',
    'Attendeecame along',
    'Other',
  ])
  const go = screen.getByRole('button', { name: 'Continue' })
  await userEvent.dblClick(go)
  await waitFor(() => expect(posts).toHaveLength(1))
  expect(posts[0].url).toBe('/api/l/open1/role')
  expect(posts[0].body).toEqual({ role: 'Attendee', hint: '', talkTitle: '', other: false })
})

test('navigates to the invite after choosing', async () => {
  mockApi()
  visit()
  await userEvent.click(await screen.findByRole('button', { name: 'Continue' }))
  // The invite page loads next; its fetch is the 404 mock above.
  await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/s/tok%209', undefined))
})

test('speaker reveals talk title', async () => {
  const posts = mockApi()
  visit()
  await screen.findByRole('radio', { name: /Speaker/ })
  expect(screen.queryByLabelText('Talk title')).toBeNull()
  await userEvent.click(screen.getByRole('radio', { name: /Speaker/ }))
  const talk = screen.getByLabelText('Talk title')
  expect(talk).toHaveAttribute('maxlength', '120')
  await userEvent.type(talk, 'Go at scale')
  await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
  await waitFor(() => expect(posts).toHaveLength(1))
  expect(posts[0].body).toEqual({ role: 'Speaker', hint: '', talkTitle: 'Go at scale', other: false })
})

test('other reveals 40-char field and sends it as hint', async () => {
  const posts = mockApi()
  visit()
  await userEvent.click(await screen.findByRole('radio', { name: 'Other' }))
  const field = screen.getByLabelText('What was your part?', { selector: 'input[type="text"]' })
  expect(field).toHaveAttribute('maxlength', '40')
  expect(field).toBeRequired()
  await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
  expect(posts).toHaveLength(0)
  await userEvent.type(field, 'Ran the A/V')
  await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
  await waitFor(() => expect(posts).toHaveLength(1))
  expect(posts[0].body).toEqual({ role: 'Other', hint: 'Ran the A/V', talkTitle: '', other: true })
})

test('rate limited state', async () => {
  const posts = mockApi({ post: [429, { error: 'rate_limited', message: 'm' }] })
  visit()
  await userEvent.click(await screen.findByRole('button', { name: 'Continue' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('try again in a minute')
  expect(screen.getByRole('radio', { name: /Attendee/ })).toBeChecked()
  expect(screen.getByRole('button', { name: 'Continue' })).toBeEnabled()
  expect(posts).toHaveLength(1)
})

test('crew full is a dead end', async () => {
  mockApi({ get: [409, { error: 'crew_full', message: 'm' }] })
  visit()
  expect(await screen.findByRole('heading', { name: 'This crew is full' })).toBeInTheDocument()
  expect(screen.getByText('The captain can raise the cap in settings.')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Continue' })).toBeNull()
})

test('crew full on continue shows the dead end', async () => {
  mockApi({ post: [409, { error: 'crew_full', message: 'm' }] })
  visit()
  await userEvent.click(await screen.findByRole('button', { name: 'Continue' }))
  expect(await screen.findByRole('heading', { name: 'This crew is full' })).toBeInTheDocument()
})

test('unknown link is dead', async () => {
  mockApi({ get: [404, { error: 'not_found', message: 'm' }] })
  visit()
  expect(await screen.findByRole('heading', { name: "This link isn't valid" })).toBeInTheDocument()
})

test('other with only spaces shows an inline error and posts nothing', async () => {
  const posts = mockApi()
  visit()
  await userEvent.click(await screen.findByRole('radio', { name: 'Other' }))
  await userEvent.type(screen.getByLabelText('What was your part?', { selector: 'input[type="text"]' }), '   ')
  await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
  expect(await screen.findByText('Say what your part was.')).toBeInTheDocument()
  expect(posts).toHaveLength(0)
})
