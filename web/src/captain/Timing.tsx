import { useEffect, useRef, useState } from 'react'
import Button from '../components/Button'
import type { Detail, Patch } from '../api'
import { clock, fromLocalInput, moment, plusMinutes, toLocalInput, zone } from './format'
import { MAX_WINDOW, MIN_WINDOW, launchAtProblem, windowProblem } from './timing-rules'

type Props = { detail: Detail; onSave: (p: Patch) => Promise<unknown> | void }

const MAX_BARS = 24
const SAVE_AFTER = 700
// A half-typed value is not a problem yet: wait this long for the typing to pause.
const ANNOUNCE_AFTER = 600

type Field = 'at' | 'win'
type Status = { kind: 'idle' | 'saving' | 'saved' | 'problem'; text: string; field?: Field }

// Launch moment and window. The summary shows what is saved; "Change" opens the
// two fields, and whatever the captain picks saves itself once it is valid.
export default function Timing({ detail, onSave }: Props) {
  const editable = detail.status === 'draft'
  const [at, setAt] = useState(toLocalInput(detail.launchAt))
  const [win, setWin] = useState(String(detail.windowMinutes))
  const [open, setOpen] = useState(!detail.launchAt)
  const [status, setStatus] = useState<Status>({ kind: 'idle', text: '' })
  const save = useRef(onSave)
  save.current = onSave

  const atChanged = at !== '' && at !== toLocalInput(detail.launchAt)
  const winChanged = win !== String(detail.windowMinutes)

  // The first thing wrong with what is typed, and which field it belongs to.
  const problem = (): Status | undefined => {
    const badAt = atChanged ? launchAtProblem(at) : undefined
    if (badAt) return { kind: 'problem', text: badAt, field: 'at' }
    const badWin = winChanged ? windowProblem(win) : undefined
    if (badWin) return { kind: 'problem', text: badWin, field: 'win' }
    return undefined
  }

  // Leaving a field announces its problem at once instead of waiting for the pause.
  const announce = () => {
    const p = problem()
    if (p) setStatus(p)
  }

  useEffect(() => {
    if (!editable) return
    const bad = problem()
    if (bad) {
      // Clear whatever was said, then speak only once the typing has paused.
      setStatus({ kind: 'idle', text: '' })
      const t = window.setTimeout(() => setStatus(bad), ANNOUNCE_AFTER)
      return () => window.clearTimeout(t)
    }
    const patch: Patch = {}
    if (atChanged) patch.launchAt = fromLocalInput(at)
    if (winChanged) patch.windowMinutes = Number(win)
    if (Object.keys(patch).length === 0) return
    setStatus({ kind: 'saving', text: 'Saving...' })
    const t = window.setTimeout(async () => {
      try {
        await save.current(patch)
        setStatus({ kind: 'saved', text: 'Saved' })
      } catch {
        setStatus({ kind: 'problem', text: 'Could not save, try again.' })
      }
    }, SAVE_AFTER)
    return () => window.clearTimeout(t)
    // problem() derives from at and win, which are listed
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [at, win, editable, detail.launchAt, detail.windowMinutes])

  const total = detail.counts.total
  const ready = detail.counts.in
  const bars = Math.min(total, MAX_BARS)
  const gap = total > 0 ? Math.max(1, Math.round(detail.windowMinutes / total)) : 0

  return (
    <section className="cap-section panel cap-timing" aria-labelledby="cap-timing">
      <h2 id="cap-timing">Timing</h2>
      {detail.releaseTag && (
        <p className="cap-muted">
          Release <a className="cap-mono" href={detail.releaseUrl}>{detail.releaseTag}</a>
        </p>
      )}
      <div className="cap-facts">
        <div className="cap-fact">
          <span className="cap-eyebrow">Launch moment</span>
          <strong className="cap-fact-value">{detail.launchAt ? moment(detail.launchAt) : 'Not set'}</strong>
          <span className="cap-muted">{zone()}</span>
        </div>
        <div className="cap-fact">
          <span className="cap-eyebrow">Window</span>
          <strong className="cap-fact-value">{detail.windowMinutes} minutes</strong>
          <span className="cap-muted">{total > 0 ? `${total} posts, ${gap} min apart` : 'slots appear as people join'}</span>
        </div>
      </div>
      {bars > 0 && detail.launchAt && (
        <div className="cap-slots">
          <div className="cap-bars" aria-hidden="true">
            {Array.from({ length: bars }, (_, i) => (
              <span key={i} className="cap-bar" data-ready={i < ready ? 'true' : undefined} />
            ))}
          </div>
          <span className="cap-axis" aria-hidden="true" />
          <div className="cap-axis-labels">
            <span>{clock(detail.launchAt)}</span>
            <span>one post every {gap} min</span>
            <span>{clock(plusMinutes(detail.launchAt, detail.windowMinutes))}</span>
          </div>
          <span>
            {ready} {ready === 1 ? 'slot' : 'slots'} ready · {Math.max(0, total - ready)} waiting on approval
          </span>
        </div>
      )}
      {editable && (
        <>
          <Button variant="secondary" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
            {open ? 'Done' : 'Change moment or window'}
          </Button>
          {open && (
            <div className="cap-form">
              <label className="cap-field">
                Launch moment
                <input
                  className="field"
                  type="datetime-local"
                  value={at}
                  aria-invalid={status.field === 'at' ? true : undefined}
                  aria-describedby={status.field === 'at' ? 'cap-timing-status' : undefined}
                  onChange={(e) => setAt(e.target.value)}
                  onBlur={announce}
                />
              </label>
              <label className="cap-field">
                Window (minutes)
                <input
                  className="field"
                  type="number"
                  min={MIN_WINDOW}
                  max={MAX_WINDOW}
                  value={win}
                  aria-invalid={status.field === 'win' ? true : undefined}
                  aria-describedby={status.field === 'win' ? 'cap-timing-status' : undefined}
                  onChange={(e) => setWin(e.target.value)}
                  onBlur={announce}
                />
              </label>
            </div>
          )}
          <p id="cap-timing-status" className={status.kind === 'problem' ? 'cap-status cap-err' : 'cap-status cap-muted'} role="status">
            {status.text}
          </p>
        </>
      )}
    </section>
  )
}
