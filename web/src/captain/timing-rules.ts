// The one place the launch timing rules live: New launch and the launch page's Timing
// card both validate with these, so the captain hears the same thing on both.
export const MIN_WINDOW = 5
export const MAX_WINDOW = 720

export const WINDOW_RULE = `The window is ${MIN_WINDOW} to ${MAX_WINDOW} minutes.`
export const FUTURE_RULE = 'Pick a time in the future.'

// Why a window value (as typed) is not acceptable, or undefined when it is.
export function windowProblem(value: string): string | undefined {
  const n = Number(value)
  if (value.trim() === '' || !Number.isInteger(n) || n < MIN_WINDOW || n > MAX_WINDOW) return WINDOW_RULE
  return undefined
}

// Why a datetime-local value is not acceptable, or undefined when it is. Empty is "not set".
export function launchAtProblem(value: string, now = Date.now()): string | undefined {
  if (!value) return undefined
  if (new Date(value).getTime() <= now) return FUTURE_RULE
  return undefined
}
