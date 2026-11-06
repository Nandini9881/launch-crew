import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'
import AccountMenu from './AccountMenu'

const menu = () => screen.getByText('Account').closest('details') as HTMLDetailsElement

test('opens from the button and closes on a click outside', async () => {
  const user = userEvent.setup()
  render(
    <div>
      <AccountMenu />
      <p>elsewhere</p>
    </div>,
  )
  await user.click(screen.getByText('Account'))
  expect(menu().open).toBe(true)
  await user.click(screen.getByText('elsewhere'))
  expect(menu().open).toBe(false)
})

test('Escape closes it and returns focus to the button', async () => {
  const user = userEvent.setup()
  render(<AccountMenu />)
  await user.click(screen.getByText('Account'))
  await user.keyboard('{Escape}')
  expect(menu().open).toBe(false)
  expect(screen.getByText('Account')).toHaveFocus()
})

test('choosing an item closes it', async () => {
  const user = userEvent.setup()
  render(<AccountMenu />)
  await user.click(screen.getByText('Account'))
  await user.click(screen.getByRole('link', { name: 'Settings' }))
  expect(menu().open).toBe(false)
})

test('the button toggles it shut', async () => {
  const user = userEvent.setup()
  render(<AccountMenu />)
  await user.click(screen.getByText('Account'))
  await user.click(screen.getByText('Account'))
  expect(menu().open).toBe(false)
})

test('Sign out posts to /auth/logout instead of following a link', () => {
  render(<AccountMenu />)
  const btn = screen.getByRole('button', { name: 'Sign out' })
  const form = btn.closest('form')
  expect(form).not.toBeNull()
  expect(form?.getAttribute('method')).toBe('post')
  expect(form?.getAttribute('action')).toBe('/auth/logout')
  expect(screen.queryByRole('link', { name: 'Sign out' })).toBeNull()
})
