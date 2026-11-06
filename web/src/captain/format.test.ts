import { copy, fromLocalInput, stateTone, toLocalInput, until } from './format'

afterEach(() => vi.unstubAllGlobals())

test('copy reports whether the clipboard took the text', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
  expect(await copy('hello')).toBe(true)
  expect(writeText).toHaveBeenCalledWith('hello')

  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
  expect(await copy('hello')).toBe(false)

  vi.stubGlobal('navigator', { ...navigator, clipboard: undefined })
  expect(await copy('hello')).toBe(false)
})

test('until counts down in minutes, hours and days, and is empty once passed', () => {
  const now = Date.parse('2026-11-06T12:00:00Z')
  const plus = (min: number) => new Date(now + min * 60_000).toISOString()
  expect(until(null, now)).toBe('')
  expect(until(plus(-1), now)).toBe('')
  expect(until(plus(0), now)).toBe('')
  expect(until(plus(38), now)).toBe('38m')
  expect(until(plus(134), now)).toBe('2h 14m')
  expect(until(plus(47 * 60), now)).toBe('47h 0m')
  expect(until(plus(3 * 24 * 60), now)).toBe('3d')
})

test('stateTone: green when in, red when out or failed, grey while waiting', () => {
  for (const s of ['approved', 'queued', 'posted', 'in']) expect(stateTone(s)).toBe('ok')
  for (const s of ['comment_failed', 'opted_out', 'declined', 'removed']) expect(stateTone(s)).toBe('err')
  for (const s of ['invited', 'opened', 'drafting', 'suggested', 'ticked']) expect(stateTone(s)).toBe('wait')
})

test('toLocalInput and fromLocalInput round-trip an instant through the datetime-local value', () => {
  const iso = new Date(2026, 10, 3, 9, 5).toISOString()
  const local = toLocalInput(iso)
  expect(local).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/)
  expect(local.endsWith('T09:05')).toBe(true)
  expect(fromLocalInput(local)).toBe(iso)
  expect(toLocalInput(null)).toBe('')
})
