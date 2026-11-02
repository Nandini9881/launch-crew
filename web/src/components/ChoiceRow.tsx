import './components.css'

type Props = {
  name: string
  value: string
  label: string
  hint?: string
  checked: boolean
  onChange: () => void
}

// A radio whose whole 56 px row is the label and the target.
export default function ChoiceRow({ name, value, label, hint, checked, onChange }: Props) {
  return (
    <label className="choice-row">
      <input type="radio" name={name} value={value} checked={checked} onChange={onChange} />
      <span className="choice-name">{label}</span>
      {hint && <span className="choice-hint">{hint}</span>}
    </label>
  )
}
