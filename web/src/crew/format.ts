const SERVICES: Record<string, string> = {
  twitter: 'X', x: 'X', bluesky: 'Bluesky', threads: 'Threads', mastodon: 'Mastodon', linkedin: 'LinkedIn',
}

export const serviceName = (s: string) => SERVICES[s.toLowerCase()] ?? s

// 12-hour clock in the viewer's timezone, without the meridiem.
export const hm = (d: Date) => `${d.getHours() % 12 || 12}:${String(d.getMinutes()).padStart(2, '0')}`
export const meridiem = (d: Date) => (d.getHours() < 12 ? 'AM' : 'PM')
export const weekday = (d: Date) => d.toLocaleDateString('en-US', { weekday: 'short' })

// "Tue 9:00"
export const when = (iso: string) => {
  const d = new Date(iso)
  return `${weekday(d)} ${hm(d)}`
}
