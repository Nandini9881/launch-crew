import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError } from '../api'
import VoiceCard from './VoiceCard'

const profile = { sentenceLength: 'short', case: 'lowercase', hashtags: 'few', extra: 'never say excited' }

test('prefilled from profile', () => {
  render(<VoiceCard profile={profile} onSave={vi.fn()} onClose={vi.fn()} />)
  expect(screen.getByRole('radio', { name: 'short' })).toBeChecked()
  expect(screen.getByRole('radio', { name: 'lowercase' })).toBeChecked()
  expect(screen.getByRole('radio', { name: 'one or two' })).toBeChecked()
  const extra = screen.getByRole('textbox', { name: 'Anything else' })
  expect(extra).toHaveValue('never say excited')
  expect(extra).toHaveAttribute('maxlength', '80')
  expect(extra).toHaveAttribute('placeholder', 'e.g. never say excited')
  expect(screen.getByRole('group', { name: 'Sentence length' })).toBeInTheDocument()
  expect(screen.getByRole('group', { name: 'Case' })).toBeInTheDocument()
  expect(screen.getByRole('group', { name: 'Hashtags' })).toBeInTheDocument()
})

test('save sends profile once and collapses on Done', async () => {
  let resolve!: () => void
  const onSave = vi.fn(() => new Promise<void>((r) => (resolve = r)))
  const onClose = vi.fn()
  render(<VoiceCard profile={profile} onSave={onSave} onClose={onClose} />)
  await userEvent.click(screen.getByRole('radio', { name: 'long' }))
  await userEvent.click(screen.getByRole('radio', { name: 'sentence case' }))
  const save = screen.getByRole('button', { name: 'Rewrite in this voice' })
  await userEvent.click(save)
  await userEvent.click(save)
  expect(onSave).toHaveBeenCalledTimes(1)
  expect(onSave).toHaveBeenCalledWith({ sentenceLength: 'long', case: 'sentence', hashtags: 'few', extra: 'never say excited' })
  expect(save).toHaveAttribute('aria-busy', 'true')
  resolve()
  expect(await screen.findByRole('status')).toBeInTheDocument()
  expect(onClose).not.toHaveBeenCalled()
  await userEvent.click(screen.getByRole('button', { name: 'Done' }))
  expect(onClose).toHaveBeenCalledTimes(1)
  expect(onSave).toHaveBeenCalledTimes(1)
})

test('escape collapses', async () => {
  const onClose = vi.fn()
  render(<VoiceCard profile={profile} onSave={vi.fn()} onClose={onClose} />)
  await userEvent.click(screen.getByRole('radio', { name: 'medium' }))
  await userEvent.keyboard('{Escape}')
  expect(onClose).toHaveBeenCalledTimes(1)
})

test('keeps the card and values open on a failed save', async () => {
  const onSave = vi.fn().mockRejectedValueOnce(new ApiError(400, 'invalid_voice', 'keep it under 80 characters'))
    .mockRejectedValueOnce(new ApiError(502, 'buffer_unavailable', 'x'))
  render(<VoiceCard profile={profile} onSave={onSave} onClose={vi.fn()} />)
  await userEvent.click(screen.getByRole('radio', { name: 'long' }))
  await userEvent.click(screen.getByRole('button', { name: 'Rewrite in this voice' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('keep it under 80 characters')
  expect(screen.getByRole('radio', { name: 'long' })).toBeChecked()
  expect(screen.getByRole('textbox', { name: 'Anything else' })).toHaveValue('never say excited')
  await userEvent.click(screen.getByRole('button', { name: 'Rewrite in this voice' }))
  expect(await screen.findByText('Buffer is not answering, try again')).toBeInTheDocument()
})
