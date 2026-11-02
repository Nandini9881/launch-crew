import { useState } from 'react'
import './components.css'

type Scheme = 'system' | 'light' | 'dark'
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
        <label key={o.value}>
          <input
            type="radio"
            name="lc-scheme"
            value={o.value}
            checked={scheme === o.value}
            onChange={() => choose(o.value)}
          />
          {o.label}
        </label>
      ))}
    </fieldset>
  )
}
