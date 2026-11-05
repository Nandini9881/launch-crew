import { render } from '@testing-library/react'
import Pass from './Pass'

test('renders the pass text and hides it from assistive tech', () => {
  const { container } = render(
    <Pass eyebrow="v2.4.0 · PR #482" name="Maya Kapoor" meta="@mayakp · Contributor" stub={{ label: 'Crew', value: '9', date: '6 Nov' }} />,
  )
  const el = container.querySelector('.pass')
  expect(el).toHaveAttribute('aria-hidden', 'true')
  expect(el).toHaveTextContent('Maya Kapoor')
  expect(el).toHaveTextContent('Crew')
})
