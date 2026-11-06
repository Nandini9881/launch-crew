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
  const px = Number(/min-height:\s*(\d+)px/.exec(rule?.[1] ?? '')?.[1])
  expect(px).toBeGreaterThanOrEqual(44)
})

test('busy button keeps its label and sets aria-busy', () => {
  render(<Button busy>Add</Button>)
  expect(screen.getByRole('button', { name: 'Add' })).toHaveAttribute('aria-busy', 'true')
})

test('busy button is aria-disabled so assistive tech knows it will not act', () => {
  render(<Button busy>Add</Button>)
  expect(screen.getByRole('button', { name: 'Add' })).toHaveAttribute('aria-disabled', 'true')
})

test('buttons follow DESIGN.md: secondary sits on surface, 52px on desktop and 54px on a phone', () => {
  const secondary = /\.btn-secondary\s*\{([^}]*)\}/.exec(css)?.[1] ?? ''
  expect(secondary).toMatch(/background-color:\s*var\(--color-surface\)/)
  expect(secondary).not.toMatch(/--color-tint/)
  const base = /\.btn\s*\{([^}]*)\}/.exec(css)?.[1] ?? ''
  expect(base).toMatch(/min-height:\s*52px/)
  // a phone-width media block raises the primary action to 54px
  const phone = /@media\s*\((?:max-)?width\s*(?:<=|:)\s*(\d+)px\)\s*\{[^@]*?\.btn\s*\{([^}]*)\}/s.exec(css)
  expect(phone).not.toBeNull()
  expect(Number(phone![1])).toBeGreaterThanOrEqual(375)
  expect(phone![2]).toMatch(/min-height:\s*54px/)
})
