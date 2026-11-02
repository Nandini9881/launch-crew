import { useState, type FormEvent } from 'react'
import Button from '../components/Button'
import type { Detail, Patch } from '../api'
import { fromLocalInput, toLocalInput } from './format'

type Props = { detail: Detail; onSave: (p: Patch) => void }

// Launch moment and window; tags and links sit in mono.
export default function Timing({ detail, onSave }: Props) {
  const [at, setAt] = useState(toLocalInput(detail.launchAt))
  const [win, setWin] = useState(String(detail.windowMinutes))
  const editable = detail.status === 'draft'

  function submit(e: FormEvent) {
    e.preventDefault()
    const p: Patch = {}
    if (at) p.launchAt = fromLocalInput(at)
    const n = Number(win)
    if (Number.isInteger(n) && n > 0) p.windowMinutes = n
    onSave(p)
  }

  return (
    <section className="cap-section" aria-labelledby="cap-timing">
      <h2 id="cap-timing">Timing</h2>
      {detail.releaseTag && (
        <p>
          Release <a className="cap-mono" href={detail.releaseUrl}>{detail.releaseTag}</a>
        </p>
      )}
      <form className="cap-form" onSubmit={submit}>
        <label className="cap-field">
          Launch moment
          <input type="datetime-local" value={at} disabled={!editable} onChange={(e) => setAt(e.target.value)} />
        </label>
        <label className="cap-field">
          Window (minutes)
          <input type="number" min={1} value={win} disabled={!editable} onChange={(e) => setWin(e.target.value)} />
        </label>
        {editable && (
          <Button variant="secondary" type="submit">
            Save timing
          </Button>
        )}
      </form>
    </section>
  )
}
