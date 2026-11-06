import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useNavigate } from 'react-router'
import App from './App'

afterEach(() => vi.unstubAllGlobals())

test('renders the product name', () => {
  render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  )
  expect(screen.getAllByText('Launch Crew').length).toBeGreaterThan(0)
})

function Go({ to }: { to: string }) {
  const navigate = useNavigate()
  return <button type="button" onClick={() => navigate(to)}>go</button>
}

test('a route change moves focus to the new page so it is announced; first load does not steal focus', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify([]), { status: 200 }))))
  const focus = vi.spyOn(HTMLElement.prototype, 'focus')
  render(
    <MemoryRouter initialEntries={['/captain']}>
      <App />
      <Go to="/captain/repos" />
    </MemoryRouter>,
  )
  await screen.findByRole('heading', { level: 1, name: 'Launches' })
  expect(document.activeElement).toBe(document.body)
  await act(async () => {
    await userEvent.click(screen.getByRole('button', { name: 'go' }))
  })
  await screen.findByRole('heading', { level: 1, name: 'Repos' })
  const main = screen.getByRole('main')
  expect(main).toHaveFocus()
  expect(main).toHaveAttribute('tabindex', '-1')
  // reduced motion off: the default focus scroll (to the top of the new page) is kept
  expect(focus).toHaveBeenLastCalledWith({ preventScroll: false })
  focus.mockRestore()
})

test('under reduced motion the focus move does not scroll', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify([]), { status: 200 }))))
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {} }))
  const focus = vi.spyOn(HTMLElement.prototype, 'focus')
  render(
    <MemoryRouter initialEntries={['/captain']}>
      <App />
      <Go to="/captain/repos" />
    </MemoryRouter>,
  )
  await screen.findByRole('heading', { level: 1, name: 'Launches' })
  await act(async () => {
    await userEvent.click(screen.getByRole('button', { name: 'go' }))
  })
  await screen.findByRole('heading', { level: 1, name: 'Repos' })
  expect(screen.getByRole('main')).toHaveFocus()
  expect(focus).toHaveBeenLastCalledWith({ preventScroll: true })
  focus.mockRestore()
})
