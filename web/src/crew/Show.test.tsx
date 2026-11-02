import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import App from '../App'

const toCanvas = vi.hoisted(() => vi.fn(() => Promise.resolve()))
vi.mock('qrcode', () => ({ default: { toCanvas } }))

afterEach(() => vi.unstubAllGlobals())

function visit() {
  render(
    <MemoryRouter initialEntries={['/l/open1/show']}>
      <App />
    </MemoryRouter>,
  )
}

test('show page: no chrome, QR of the short link, download link', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({ title: 'GoMeetup Nov', shortUrl: 'https://lc.test/l/open1' }),
        ),
      ),
    ),
  )
  render(
    <MemoryRouter initialEntries={['/l/open1/show']}>
      <App />
    </MemoryRouter>,
  )
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('GoMeetup Nov')
  expect(screen.getByText('https://lc.test/l/open1')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Download PNG' })).toHaveAttribute('href', '/api/l/open1/show.png')
  expect(screen.queryByRole('radiogroup')).toBeNull()
  expect(toCanvas).toHaveBeenCalledWith(expect.anything(), 'https://lc.test/l/open1', expect.objectContaining({ errorCorrectionLevel: 'H' }))
})

test('show page uses the show endpoint and notes a failed QR draw', async () => {
  toCanvas.mockRejectedValueOnce(new Error('boom'))
  const f = vi.fn(() => Promise.resolve(new Response(JSON.stringify({ title: 'T', shortUrl: 'https://lc.test/l/open1' }))))
  vi.stubGlobal('fetch', f)
  visit()
  expect(await screen.findByText(/QR code could not be drawn/)).toBeInTheDocument()
  expect(f).toHaveBeenCalledWith('/api/l/open1/show', undefined)
})
