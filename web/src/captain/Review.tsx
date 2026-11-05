import Button from '../components/Button'
import { serviceName } from '../crew/format'
import type { Detail, Pr } from '../api'
import { limitOf } from './format'

type Props = { detail: Detail; pending: boolean; onHidden: (hidden: number[]) => void }

function PrRow({ pr, editable, pending, toggle }: { pr: Pr; editable: boolean; pending: boolean; toggle: () => void }) {
  return (
    <li className="cap-pr">
      <span>
        <span className="cap-mono">#{pr.prNumber}</span> {pr.title}
        <span className="cap-muted"> · {pr.authorLogin}</span>
      </span>
      {editable && (
        <Button variant="ghost" aria-pressed={pr.hidden} disabled={pending} onClick={toggle}>
          {pr.hidden ? 'Keep' : 'Hide'}
          <span className="visually-hidden"> #{pr.prNumber}</span>
        </Button>
      )}
    </li>
  )
}

// Left column: headline changes, the captain's own posts, hidden chores.
export default function Review({ detail, pending, onHidden }: Props) {
  const editable = detail.status === 'draft'
  const all = [...detail.headlines, ...detail.chores]
  const toggle = (n: number) => {
    const hidden = new Set(all.filter((p) => p.hidden).map((p) => p.prNumber))
    if (!hidden.delete(n)) hidden.add(n)
    onHidden([...hidden])
  }
  const shown = detail.headlines.filter((p) => !p.hidden)
  const hiddenPrs = all.filter((p) => p.hidden)
  const posts = detail.captainDrafts

  return (
    <div className="cap-col">
      {detail.source === 'github' && (
        <section className="cap-section panel" aria-labelledby="cap-headlines">
          <h2 id="cap-headlines">Headline changes</h2>
          {shown.length === 0 ? (
            <p className="cap-muted">No headline changes left. Keep one from the hidden list.</p>
          ) : (
            <ul className="cap-list">
              {shown.map((p) => (
                <PrRow key={p.prNumber} pr={p} editable={editable} pending={pending} toggle={() => toggle(p.prNumber)} />
              ))}
            </ul>
          )}
        </section>
      )}
      <section className="cap-section panel" aria-labelledby="cap-posts">
        <h2 id="cap-posts">Your posts</h2>
        {posts.length === 0 ? (
          <p className="cap-muted">
            {detail.bufferConnected ? 'Building your posts...' : 'Connect Buffer to see your posts here.'}
          </p>
        ) : (
          <ul className="cap-list">
            {posts.map((d) => (
              <li key={d.id} className="cap-post">
                <div className="cap-post-head">
                  <strong>{d.channelName}</strong>
                  <span className={d.status === 'failed' ? 'cap-muted cap-err' : 'cap-muted'}>
                    {serviceName(d.service)} · {d.status === 'draft' && d.bufferPostId ? 'saved as draft' : d.status}
                  </span>
                </div>
                {d.note && <p className="cap-muted cap-post-note">{d.note}</p>}
                <p className="cap-post-text">{d.text}</p>
                {limitOf(d.service) && (
                  <span className="cap-count-line cap-muted">
                    {d.text.length} / {limitOf(d.service)}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
      {hiddenPrs.length > 0 && (
        <details className="cap-chores panel">
          <summary>
            {hiddenPrs.length} hidden {hiddenPrs.length === 1 ? 'chore' : 'chores'}
          </summary>
          <ul className="cap-list">
            {hiddenPrs.map((p) => (
              <PrRow key={p.prNumber} pr={p} editable={editable} pending={pending} toggle={() => toggle(p.prNumber)} />
            ))}
          </ul>
        </details>
      )}
    </div>
  )
}
