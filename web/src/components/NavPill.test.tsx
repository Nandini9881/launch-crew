import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router'
import NavPill from './NavPill'

function Where() {
  return <p data-testid="where">{useLocation().pathname}</p>
}

test('the wordmark is an in-app link: no full reload on the way home', async () => {
  render(
    <MemoryRouter initialEntries={['/captain/repos']}>
      <NavPill home="/captain" />
      <Where />
    </MemoryRouter>,
  )
  const home = screen.getByRole('link', { name: /Launch Crew/ })
  expect(home).toHaveAttribute('href', '/captain')
  await userEvent.click(home)
  expect(screen.getByTestId('where')).toHaveTextContent(/^\/captain$/)
})
