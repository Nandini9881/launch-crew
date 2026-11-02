import { render, screen } from '@testing-library/react'
import Chip from './Chip'

test('chip is a labelled checkbox with reason when disabled', () => {
  render(<Chip label="YouTube" checked={false} onChange={() => {}} disabled reason="no text posts" />)
  const box = screen.getByLabelText('YouTube · no text posts')
  expect(box).toBeDisabled()
  expect(box).toHaveAttribute('type', 'checkbox')
})
