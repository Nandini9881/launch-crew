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

// "2h 14m", "38m" or "3d" until an instant; empty once it has passed.
export const until = (iso: string | null, now = Date.now()) => {
  if (!iso) return ''
  const m = Math.round((new Date(iso).getTime() - now) / 60000)
  if (m <= 0) return ''
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  return h < 48 ? `${h}h ${m % 60}m` : `${Math.round(h / 24)}d`
}

// Buffer's character limits per service, for the counter on the captain's own posts.
const LIMITS: Record<string, number> = { twitter: 280, x: 280, bluesky: 300, threads: 500, mastodon: 500, linkedin: 3000 }
export const limitOf = (service: string) => LIMITS[service.toLowerCase()]

export const initials = (name: string) =>
  name
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join('')

// Tone for a crew state word: green when in, red when out or failed, grey while waiting.
export const stateTone = (state: string): 'ok' | 'err' | 'wait' =>
  /approved|queued|posted|in$/.test(state) ? 'ok' : /failed|opted|declined|removed/.test(state) ? 'err' : 'wait'

const tz = () => Intl.DateTimeFormat().resolvedOptions().timeZone

export const zone = () => tz().replace(/_/g, ' ')

// 24-hour clock time of an instant, as the timing card shows it.
export const clock = (iso: string) =>
  new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })

// "Thu 6 Nov, 14:30"
export const moment = (iso: string) => {
  const d = new Date(iso)
  const day = d.toLocaleDateString([], { weekday: 'short', day: 'numeric', month: 'short' })
  return `${day}, ${clock(iso)}`
}

export const plusMinutes = (iso: string, minutes: number) => new Date(new Date(iso).getTime() + minutes * 60000).toISOString()
