import { render, screen } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import Button from './Button'

// jsdom does not apply stylesheets, so check the class the component renders
// and the declaration in the stylesheet that class resolves to.
const css = readFileSync('src/components/components.css', 'utf8')

test('primary button is at least 44px tall', () => {
  render(<Button variant="primary">Launch</Button>)
  const btn = screen.getByRole('button', { name: 'Launch' })
  expect(btn).toHaveClass('btn', 'btn-primary')
  const rule = /\.btn\s*\{([^}]*)\}/.exec(css)
  expect(rule?.[1]).toMatch(/min-height:\s*44px/)
})

test('busy button keeps its label and sets aria-busy', () => {
  render(<Button busy>Add</Button>)
  expect(screen.getByRole('button', { name: 'Add' })).toHaveAttribute('aria-busy', 'true')
})
