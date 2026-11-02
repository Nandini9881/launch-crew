import { ApiError } from '../api'
import { hm, meridiem, weekday } from '../crew/format'

// "Tue 9:00 AM" in the viewer's timezone.
export const stamp = (iso: string) => {
  const d = new Date(iso)
  return `${weekday(d)} ${hm(d)} ${meridiem(d)}`
}

const pad = (n: number) => String(n).padStart(2, '0')

// ISO instant to the value a datetime-local input expects.
export const toLocalInput = (iso: string | null) => {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export const fromLocalInput = (v: string) => new Date(v).toISOString()

export const copy = (text: string) => navigator.clipboard?.writeText(text).catch(() => undefined)

export const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, 'error', String(e)))
