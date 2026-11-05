import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import NewLaunch from './NewLaunch'

function setup() {
  const calls: { method: string; url: string; body?: Record<string, unknown> }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      calls.push({ method: init?.method ?? 'GET', url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      return Promise.resolve(new Response(JSON.stringify({ id: 'new1' }), { status: 201 }))
    }),
  )
  render(
    <MemoryRouter>
      <NewLaunch />
    </MemoryRouter>,
  )
  return calls
}

test('asks where the launch comes from, meetup selected by default', () => {
  setup()
  expect(screen.getByText('Where does this come from?')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Meetup or talk' })).toHaveAttribute('aria-pressed', 'true')
  expect(screen.getByRole('button', { name: 'GitHub release' })).toHaveAttribute('aria-pressed', 'false')
})

test('GitHub release shows the explainer and no form to submit', async () => {
  setup()
  await userEvent.click(screen.getByRole('button', { name: 'GitHub release' }))
  expect(screen.getByText('Launch Crew starts this on its own.')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Add a repo' })).toHaveAttribute('href', '/captain/repos')
  expect(screen.getByRole('link', { name: 'See your repos' })).toHaveAttribute('href', '/captain/repos')
  expect(screen.queryByRole('button', { name: 'Create launch' })).toBeNull()
  expect(screen.queryByLabelText('Title')).toBeNull()
})

test('Meetup prefills the three usual roles; Manual post starts bare', async () => {
  setup()
  const names = () => screen.queryAllByLabelText(/^Role \d name$/).map((e) => (e as HTMLInputElement).value)
  expect(names()).toEqual(['Attendee', 'Speaker', 'Organiser'])
  await userEvent.click(screen.getByRole('button', { name: 'Manual post' }))
  expect(names()).toEqual([])
  await userEvent.click(screen.getByRole('button', { name: 'Meetup or talk' }))
  expect(names()).toEqual(['Attendee', 'Speaker', 'Organiser'])
})

test('the default QR mark is the plane and sends nothing extra', async () => {
  const calls = setup()
  expect(screen.getByRole('radio', { name: 'Plane' })).toBeChecked()
  await userEvent.type(screen.getByLabelText('Title'), 'Go night')
  await userEvent.click(screen.getByRole('button', { name: 'Create launch' }))
  const post = calls.find((c) => c.method === 'POST')
  expect(post?.body).toBeDefined()
  expect(post?.body).not.toHaveProperty('qrLogo')
  expect(post?.body).not.toHaveProperty('qrLogoUrl')
})

test('choosing None sends qrLogo none', async () => {
  const calls = setup()
  await userEvent.click(screen.getByRole('radio', { name: 'None' }))
  await userEvent.type(screen.getByLabelText('Title'), 'Go night')
  await userEvent.click(screen.getByRole('button', { name: 'Create launch' }))
  expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ qrLogo: 'none' })
})

test('my own image needs an https URL, then sends qrLogo custom with it', async () => {
  const calls = setup()
  await userEvent.click(screen.getByRole('radio', { name: 'My own image' }))
  await userEvent.type(screen.getByLabelText('Title'), 'Go night')
  await userEvent.type(screen.getByLabelText('QR mark image URL'), 'http://x.test/a.png')
  await userEvent.click(screen.getByRole('button', { name: 'Create launch' }))
  expect(await screen.findByText('Use an https:// image URL.')).toBeInTheDocument()
  expect(calls.some((c) => c.method === 'POST')).toBe(false)

  const url = screen.getByLabelText('QR mark image URL')
  await userEvent.clear(url)
  await userEvent.type(url, 'https://x.test/logo.png')
  await userEvent.click(screen.getByRole('button', { name: 'Create launch' }))
  expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ qrLogo: 'custom', qrLogoUrl: 'https://x.test/logo.png' })
})
