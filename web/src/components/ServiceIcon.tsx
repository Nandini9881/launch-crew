import type { ReactNode } from 'react'
import './components.css'

// Small outline marks for the networks Buffer posts to. Decorative: the network's name always sits beside it.
const MARKS: Record<string, ReactNode> = {
  twitter: <path d="M3 3 11 11M11 3 3 11" />,
  linkedin: (
    <>
      <path d="M3.5 6V11" />
      <circle cx="3.5" cy="3.4" r="0.6" />
      <path d="M6.7 11V7.8a1.9 1.9 0 0 1 3.8 0V11" />
    </>
  ),
  instagram: (
    <>
      <rect x="2" y="2" width="10" height="10" rx="3" />
      <circle cx="7" cy="7" r="2.3" />
      <circle cx="10" cy="4" r="0.4" />
    </>
  ),
  facebook: <path d="M8.6 12.5V7.4h1.8l.3-2H8.6V4.2c0-.6.3-.9 1-.9h1.2V1.5C10.5 1.4 9.9 1.4 9.3 1.4 7.7 1.4 6.7 2.4 6.7 4v1.4H5v2h1.7v5.1" />,
  threads: (
    <>
      <circle cx="7" cy="7" r="2" />
      <path d="M9 7v.8a1.6 1.6 0 0 0 3.2 0V7a5.2 5.2 0 1 0-2 4.1" />
    </>
  ),
  bluesky: (
    <path d="M7 6.3C6.3 4.8 4.3 2.5 2.5 2.5c-.6 0-.8.4-.8 1.3 0 .6.2 2 .4 2.6.4 1 1.5 1.3 2.4 1.1-1.7.3-3.2 1-1.2 2.9 1.9 1.9 2.7-.4 2.7-1.2.1.8 1 3.1 2.8 1.2 2-1.9.5-2.6-1.2-2.9.9.2 2-.1 2.4-1.1.2-.6.4-2 .4-2.6 0-.9-.2-1.3-.8-1.3C9.8 2.5 7.7 4.8 7 6.3z" />
  ),
  mastodon: <path d="M2.5 5.2c0-1.4.9-2.2 2.3-2.2h4.4c1.4 0 2.3.8 2.3 2.2V8c0 1.4-.9 2.2-2.3 2.2H5L2.5 12z" />,
}

const ALIAS: Record<string, string> = { x: 'twitter' }

export default function ServiceIcon({ service }: { service: string }) {
  const key = ALIAS[service.toLowerCase()] ?? service.toLowerCase()
  return (
    <svg className="service-icon" viewBox="0 0 14 14" aria-hidden="true">
      {MARKS[key] ?? (
        <>
          <circle cx="7" cy="7" r="4.5" />
          <circle cx="7" cy="7" r="1" />
        </>
      )}
    </svg>
  )
}
