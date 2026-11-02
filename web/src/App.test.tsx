import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import App from './App'

test('renders the product name', () => {
  render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  )
  expect(screen.getAllByText('Launch Crew').length).toBeGreaterThan(0)
})
