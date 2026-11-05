import { render, screen } from '@testing-library/react'
import Loader from './Loader'

test('announces its label as a status', () => {
  render(<Loader label="Building your post" />)
  const el = screen.getByRole('status')
  expect(el).toHaveClass('loader')
  expect(el).toHaveTextContent('Building your post')
})

test('hides the flight path from assistive tech and shows the hint', () => {
  const { container } = render(<Loader label="Working" hint="Nothing is queued yet." />)
  expect(container.querySelector('.loader-track')).toHaveAttribute('aria-hidden', 'true')
  expect(screen.getByText('Nothing is queued yet.')).toHaveClass('loader-hint')
})

test('compact sets its data flag', () => {
  render(<Loader label="Working" compact />)
  expect(screen.getByRole('status')).toHaveAttribute('data-compact', 'true')
})
