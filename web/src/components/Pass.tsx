import './components.css'

type Props = {
  eyebrow: string
  name: string
  meta: string
  /** Small label, big value and a date on the stub, e.g. Crew / 9 / 6 Nov. */
  stub: { label: string; value: string; date: string }
  footer?: string
}

// The contributor card as live markup: an ink pass with a perforated stub.
// Decorative: the real card image carries the information, so this is hidden from assistive tech.
export default function Pass({ eyebrow, name, meta, stub, footer }: Props) {
  return (
    <div className="pass" aria-hidden="true">
      <div className="pass-main">
        <span className="pass-eyebrow">{eyebrow}</span>
        <span className="pass-name">{name}</span>
        <span className="pass-meta">{meta}</span>
        {footer && <span className="pass-footer">{footer}</span>}
      </div>
      <div className="perf perf-v" />
      <div className="pass-stub">
        <span className="pass-stub-label">{stub.label}</span>
        <span className="pass-stub-value">{stub.value}</span>
        <span className="pass-stub-date">{stub.date}</span>
      </div>
    </div>
  )
}
