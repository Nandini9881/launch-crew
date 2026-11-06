import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import type { Repo } from '../api'
import Repos from './Repos'

const repo = (over: Partial<Repo> = {}): Repo => ({ id: 'r1', fullName: 'acme/rocket', enabled: true, allowPrerelease: false, ...over })

function setup(list: Repo[]) {
  const calls: { method: string; url: string; body?: unknown }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET'
      const body = init?.body ? JSON.parse(String(init.body)) : undefined
      calls.push({ method, url, body })
      if (method === 'PATCH') return Promise.resolve(new Response(JSON.stringify({ ...list[0], ...body }), { status: 200 }))
      return Promise.resolve(new Response(JSON.stringify(list), { status: 200 }))
    }),
  )
  render(
    <MemoryRouter>
      <Repos />
    </MemoryRouter>,
  )
  return calls
}

test('Add a repo is the install link, in the header', async () => {
  setup([repo()])
  await screen.findByText('acme/rocket')
  expect(screen.getByRole('link', { name: 'Add a repo' })).toHaveAttribute('href', '/github/install/start')
  expect(screen.queryByRole('link', { name: /install the github app/i })).toBeNull()
})

test('each repo has two switches that call patchRepo', async () => {
  const calls = setup([repo()])
  const watch = await screen.findByRole('switch', { name: 'acme/rocket: watch releases' })
  const pre = screen.getByRole('switch', { name: 'acme/rocket: include prereleases' })
  expect(watch).toBeChecked()
  expect(pre).not.toBeChecked()

  await userEvent.click(pre)
  expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ allowPrerelease: true })
  await userEvent.click(watch)
  expect(calls.filter((c) => c.method === 'PATCH').at(-1)?.body).toEqual({ enabled: false })
})

test('no repos shows a friendly empty state with the add button', async () => {
  setup([])
  expect(await screen.findByRole('heading', { name: 'No repos yet' })).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: 'Add a repo' }).length).toBeGreaterThan(0)
  expect(screen.getByText(/creates a draft launch/)).toBeInTheDocument()
})

test('offline shows a plain message and Retry fetches again', async () => {
  const fetchMock = vi.fn()
    .mockRejectedValueOnce(new TypeError('Failed to fetch'))
    .mockResolvedValueOnce(new Response(JSON.stringify([repo()]), { status: 200 }))
  vi.stubGlobal('fetch', fetchMock)
  render(
    <MemoryRouter>
      <Repos />
    </MemoryRouter>,
  )
  expect(await screen.findByRole('alert')).toHaveTextContent('Could not reach Launch Crew. Check your connection and try again.')
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByText('acme/rocket')).toBeInTheDocument()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(fetchMock).toHaveBeenCalledTimes(2)
})
