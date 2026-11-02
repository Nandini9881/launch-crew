import { render, screen } from '@testing-library/react'
import Alert from './Alert'

test.each([
  ['info', 'status'],
  ['success', 'status'],
  ['warning', 'status'],
  ['error', 'alert'],
] as const)('alert %s has role %s and its tone class', (tone, role) => {
  render(<Alert tone={tone} lead="Note">text</Alert>)
  const el = screen.getByRole(role)
  expect(el).toHaveClass('alert', `alert-${tone}`)
  expect(screen.getByText('Note')).toHaveClass('alert-lead')
})
