import Alert from '../components/Alert'
import Button from '../components/Button'
import Row from '../components/Row'
import type { Check } from '../api'

type Props = {
  checks: Check[]
  launched: boolean
  busy: boolean
  error?: string
  onLaunch: () => void
}

export const redCount = (checks: Check[]) => checks.filter((c) => !c.ok).length

// Seven checks; a red row names its fix. Launch stays disabled until none are red.
export default function Preflight({ checks, launched, busy, error, onLaunch }: Props) {
  const red = redCount(checks)
  const label = launched ? 'Launched' : red > 0 ? `Launch · ${red} check${red === 1 ? '' : 's'} left` : 'Launch'
  return (
    <section className="cap-section cap-preflight panel" aria-labelledby="cap-preflight">
      <div className="cap-section-head">
        <h2 id="cap-preflight">Preflight</h2>
        {checks.length > 0 && (
          <span className={red > 0 ? 'cap-chip cap-chip-warn' : 'cap-chip cap-chip-ok'}>
            {checks.length - red}/{checks.length} ready
          </span>
        )}
      </div>
      {checks.length > 0 && (
        <ol className="cap-list">
          {checks.map((c) => (
            <Row key={c.id} name={c.name} state={c.ok ? 'ready' : 'needs a fix'} tone={c.ok ? 'ok' : 'err'} fix={c.ok ? undefined : c.fix} />
          ))}
        </ol>
      )}
      {error && <Alert tone="error">{error}</Alert>}
      <Button variant="primary" className="cap-launch-btn" disabled={launched || red > 0 || busy} busy={busy} onClick={onLaunch}>
        {label}
      </Button>
      {!launched && <p className="cap-muted cap-reassure">Nothing posts until each person taps approve.</p>}
    </section>
  )
}
