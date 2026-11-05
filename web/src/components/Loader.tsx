import './components.css'

type LoaderProps = {
  /** What is happening, in words. Announced to screen readers. */
  label: string
  /** One calmer line under the label, e.g. what the user can expect. */
  hint?: string
  /** Smaller track and type, for use inside a card or next to a button. */
  compact?: boolean
}

// A plane flies once along a dashed flight path, leaving a solid trail, then
// the loop restarts. The path reuses the perforation motif from the contributor
// card. With reduced motion the plane rests mid-route.
export default function Loader({ label, hint, compact }: LoaderProps) {
  return (
    <div className="loader" role="status" data-compact={compact || undefined}>
      <div className="loader-track" aria-hidden="true">
        <span className="loader-stop" />
        <span className="loader-line" />
        <span className="loader-trail" />
        <span className="loader-stop loader-stop-end" />
        <span className="loader-flight">
          <svg className="loader-plane" viewBox="0 0 24 24">
            <g transform="rotate(90 12 12)">
              <path d="M12 2.2c1.05 0 1.85 1.3 1.85 3v4.1l7 3.9v2.1l-7 -2v3.5l2.35 1.85v1.55L12 19.1l-4.2 1.2v-1.55l2.35 -1.85v-3.5l-7 2v-2.1l7 -3.9V5.2c0 -1.7 0.8 -3 1.85 -3z" />
            </g>
          </svg>
        </span>
      </div>
      <p className="loader-label">{label}</p>
      {hint && <p className="loader-hint">{hint}</p>}
    </div>
  )
}
