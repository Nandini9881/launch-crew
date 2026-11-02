import { useState } from 'react'
import Button from '../components/Button'
import type { CrewRow, Detail } from '../api'
import { copy } from './format'

type Props = { detail: Detail; pending: boolean; onTicked: (ids: string[]) => void }

const pickable = (c: CrewRow) => c.state === 'suggested' || c.state === 'ticked'

function CopyLink({ url, label }: { url: string; label: string }) {
  const [done, setDone] = useState(false)
  return (
    <Button
      variant="ghost"
      onClick={async () => {
        await copy(url)
        setDone(true)
      }}
    >
      {done ? 'Copied' : label}
    </Button>
  )
}

// Right column, first section: who is in, with states; add by link for manual launches.
export default function Crew({ detail, pending, onTicked }: Props) {
  const draft = detail.status === 'draft'
  const toggle = (id: string) => {
    const ticked = new Set(detail.crew.filter((c) => c.state === 'ticked').map((c) => c.id))
    if (!ticked.delete(id)) ticked.add(id)
    onTicked([...ticked])
  }
  return (
    <section className="cap-section" aria-labelledby="cap-crew">
      <h2 id="cap-crew">Crew</h2>
      <p className="cap-muted">
        {detail.counts.in} of {detail.counts.total} in
        {detail.openCount > 0 && ` · ${detail.openCount} scanned`}
      </p>
      {detail.crew.length === 0 ? (
        <p className="cap-muted">
          {detail.source === 'github' ? 'No contributors found in this release.' : 'No one has joined yet.'}
        </p>
      ) : (
        <ul className="cap-list">
          {detail.crew.map((c) => (
            <li key={c.id} className="cap-crew-row">
              {draft && pickable(c) ? (
                <label className="cap-tick">
                  <input type="checkbox" checked={c.state === 'ticked'} disabled={pending} onChange={() => toggle(c.id)} />
                  {c.name}
                </label>
              ) : (
                <span>{c.name}</span>
              )}
              <span className="cap-muted">{c.role}</span>
              {c.commentFailed ? (
                c.inviteUrl ? (
                  <CopyLink url={c.inviteUrl} label="comment failed · copy link" />
                ) : (
                  <span className="cap-state">comment failed</span>
                )
              ) : (
                <span className="cap-state">{c.state.replaceAll("_", " ")}</span>
              )}
            </li>
          ))}
        </ul>
      )}
      {detail.openUrl && (
        <div className="cap-addlink">
          <label>
            Add by link
            <input readOnly value={detail.openUrl} onFocus={(e) => e.currentTarget.select()} />
          </label>
          <CopyLink url={detail.openUrl} label="Copy link" />
        </div>
      )}
    </section>
  )
}
