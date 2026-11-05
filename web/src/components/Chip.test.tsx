import { render, screen } from '@testing-library/react'
import Chip from './Chip'

test('chip is a labelled checkbox with reason when disabled', () => {
  render(<Chip label="YouTube" checked={false} onChange={() => {}} disabled reason="no text posts" />)
  const box = screen.getByLabelText('YouTube · no text posts')
  expect(box).toBeDisabled()
  expect(box).toHaveAttribute('type', 'checkbox')
})

test('chip names the network beside the account and keeps the icon decorative', () => {
  const { container } = render(
    <Chip label="JobHigh5" meta="LinkedIn" icon={<svg />} checked onChange={() => {}} />,
  )
  expect(screen.getByLabelText('JobHigh5 · LinkedIn')).toBeChecked()
  expect(container.querySelector('.chip-icon')).toHaveAttribute('aria-hidden', 'true')
  expect(container.querySelector('.chip-meta')).toHaveTextContent('LinkedIn')
})
