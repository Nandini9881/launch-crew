import { useState, type ReactNode } from 'react'
import './components.css'

type Scheme = 'system' | 'light' | 'dark'
const ICONS: Record<Scheme, ReactNode> = {
  system: (
    <>
      <rect x="3" y="4" width="18" height="12" rx="2" />
      <path d="M8 20h8M12 16v4" />
    </>
  ),
  light: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4" />
    </>
  ),
  dark: <path d="M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z" />,
}

const options: { value: Scheme; label: string }[] = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

function stored(): Scheme {
  try {
    const v = localStorage.getItem('lc.scheme')
    if (v === 'light' || v === 'dark') return v
  } catch {
    /* storage blocked */
  }
  return 'system'
}

export default function SchemeSwitch() {
  const [scheme, setScheme] = useState<Scheme>(stored)

  function choose(next: Scheme) {
    setScheme(next)
    if (next === 'system') delete document.documentElement.dataset.scheme
    else document.documentElement.dataset.scheme = next
    try {
      if (next === 'system') localStorage.removeItem('lc.scheme')
      else localStorage.setItem('lc.scheme', next)
    } catch {
      /* attribute is already applied for this visit */
    }
  }

  return (
    <fieldset className="scheme-switch">
      <legend className="visually-hidden">Appearance</legend>
      {options.map((o) => (
        <label key={o.value} title={o.label}>
          <input
            type="radio"
            name="lc-scheme"
            value={o.value}
            checked={scheme === o.value}
            onChange={() => choose(o.value)}
          />
          <svg viewBox="0 0 24 24" aria-hidden="true">
            {ICONS[o.value]}
          </svg>
          <span className="visually-hidden">{o.label}</span>
        </label>
      ))}
    </fieldset>
  )
}
