import { FUTURE_RULE, MAX_WINDOW, MIN_WINDOW, WINDOW_RULE, launchAtProblem, windowProblem } from './timing-rules'

test('the window is 5 to 720 whole minutes', () => {
  expect(MIN_WINDOW).toBe(5)
  expect(MAX_WINDOW).toBe(720)
  expect(windowProblem('90')).toBeUndefined()
  expect(windowProblem('5')).toBeUndefined()
  expect(windowProblem('720')).toBeUndefined()
  for (const bad of ['4', '721', '0', '-5', '1.5', 'abc', '']) expect(windowProblem(bad)).toBe(WINDOW_RULE)
  expect(WINDOW_RULE).toBe('The window is 5 to 720 minutes.')
})

test('a launch moment must be in the future; empty means not set', () => {
  const now = Date.parse('2026-11-06T12:00:00')
  expect(launchAtProblem('', now)).toBeUndefined()
  expect(launchAtProblem('2026-11-06T12:30', now)).toBeUndefined()
  expect(launchAtProblem('2026-11-06T12:00', now)).toBe(FUTURE_RULE)
  expect(launchAtProblem('2020-01-01T10:00', now)).toBe(FUTURE_RULE)
  expect(FUTURE_RULE).toBe('Pick a time in the future.')
})
