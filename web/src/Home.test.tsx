import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import App from './App'

test('home offers sign in and the settings link', () => {
  render(
    <MemoryRouter initialEntries={['/']}>
      <App />
    </MemoryRouter>,
  )
  expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Sign in with Buffer' })).toHaveAttribute('href', '/auth/buffer/start?return_to=/captain')
  expect(screen.getByRole('link', { name: 'Opt out or delete your data' })).toHaveAttribute('href', '/settings')
  expect(screen.getAllByRole('main')).toHaveLength(1)
})
