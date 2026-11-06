import { readFileSync } from 'node:fs'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import Home from './Home'

test('the final sign-in is a real secondary button, not an ad hoc variant', () => {
  render(
    <MemoryRouter>
      <Home />
    </MemoryRouter>,
  )
  const links = screen.getAllByRole('link', { name: 'Sign in with Buffer' })
  const final = links.find((l) => l.classList.contains('home-final-btn')) ?? links.at(-1)!
  expect(final).toHaveClass('btn', 'btn-secondary')
  const css = readFileSync('src/home.css', 'utf8')
  const rule = /\.home-final-btn\s*\{([^}]*)\}/.exec(css)?.[1] ?? ''
  expect(rule).not.toMatch(/background-color|color:/)
  expect(css).not.toMatch(/\.home-final-btn:hover/)
})
