import './components.css'

type Props = {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  reason?: string
}

export default function Chip({ label, checked, onChange, disabled, reason }: Props) {
  const name = disabled && reason ? `${label} · ${reason}` : label
  return (
    <label className="chip">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        aria-label={name}
      />
      {name}
    </label>
  )
}
