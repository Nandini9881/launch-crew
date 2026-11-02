import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, vi } from 'vitest'
import SchemeSwitch from './SchemeSwitch'

afterEach(() => {
  vi.restoreAllMocks()
})

test('scheme switch persists and sets data-scheme', async () => {
  render(<SchemeSwitch />)
  await userEvent.click(screen.getByLabelText('Dark'))
  expect(document.documentElement.dataset.scheme).toBe('dark')
  expect(localStorage.getItem('lc.scheme')).toBe('dark')
})

test('scheme switch survives storage throwing', async () => {
  const spy = vi.spyOn(Object.getPrototypeOf(localStorage), 'setItem').mockImplementation(() => {
    throw new Error('blocked')
  })
  render(<SchemeSwitch />)
  await userEvent.click(screen.getByLabelText('Light'))
  expect(spy).toHaveBeenCalled()
  expect(document.documentElement.dataset.scheme).toBe('light')
})

test('choosing System removes the attribute and the key', async () => {
  localStorage.setItem('lc.scheme', 'dark')
  document.documentElement.dataset.scheme = 'dark'
  render(<SchemeSwitch />)
  await userEvent.click(screen.getByLabelText('System'))
  expect(document.documentElement.dataset.scheme).toBeUndefined()
  expect(localStorage.getItem('lc.scheme')).toBeNull()
})

test('the stored value is reflected on mount', () => {
  localStorage.setItem('lc.scheme', 'dark')
  render(<SchemeSwitch />)
  expect(screen.getByLabelText('Dark')).toBeChecked()
  expect(screen.getByLabelText('System')).not.toBeChecked()
})
