import type { ReactNode } from 'react'
import './components.css'

type Props = {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  reason?: string
  /** Decorative mark before the label, e.g. a network icon. */
  icon?: ReactNode
  /** Secondary text after the label, e.g. the network's name. */
  meta?: string
}

export default function Chip({ label, checked, onChange, disabled, reason, icon, meta }: Props) {
  const why = disabled && reason ? reason : ''
  const name = [label, meta, why].filter(Boolean).join(' · ')
  return (
    <label className="chip">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        aria-label={name}
      />
      {icon && <span className="chip-icon" aria-hidden="true">{icon}</span>}
      <span>{label}</span>
      {meta && <span className="chip-meta">{meta}</span>}
      {why && <span className="chip-meta">{why}</span>}
    </label>
  )
}
