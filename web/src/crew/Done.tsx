import { useEffect, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router'
import { getDrafts, remember, type Outcome } from '../api'
import Row from '../components/Row'
import Shell from '../components/Shell'
import Skeleton from '../components/Skeleton'
import Wordmark from '../components/Wordmark'
import Board from './Board'
import { hm, meridiem, serviceName, weekday, when } from './format'
import { Dead, Foreign, Footer, useInvite } from './shared'

const BUFFER = 'https://publish.buffer.com'

function headline(outcomes: Outcome[]) {
  const ok = outcomes.filter((o) => o.status !== 'failed')
  const times = ok.filter((o) => o.status === 'queued' && o.when).map((o) => new Date(o.when!))
  if (times.length) {
    const first = times.reduce((a, b) => (b < a ? b : a))
    return `Queued for ${weekday(first)} ${hm(first)} ${meridiem(first)}`
  }
  if (ok.some((o) => o.status === 'queued')) return 'Added to your queue'
  if (ok.length) return 'Saved as a draft'
  return ''
}

// Meridiems only when the two times straddle noon or midnight.
function caption(launchAt: string, minutes: number) {
  const a = new Date(launchAt)
  const b = new Date(a.getTime() + minutes * 60_000)
  const same = meridiem(a) === meridiem(b)
  const span = same ? `${hm(a)} and ${hm(b)}` : `${hm(a)} ${meridiem(a)} and ${hm(b)} ${meridiem(b)}`
  return `Posts land between ${span} so the timeline fills in instead of a wall.`
}

export default function Done() {
  const token = useParams().token ?? ''
  const { view, error } = useInvite(token)
  const given = (useLocation().state as { outcomes?: Outcome[] } | null)?.outcomes
  const [fetched, setFetched] = useState<Outcome[]>()
  const [saved, setSaved] = useState(false)
  const [saveFailed, setSaveFailed] = useState(false)

  const memberId = view?.member.id
  const ok = view !== undefined && !view.foreign

  // Reload: no router state, so rebuild the lines from the member's drafts.
  useEffect(() => {
    if (given?.length || !ok || !memberId) return
    let live = true
    getDrafts(memberId, token).then(
      (r) => live && setFetched(r.drafts
        // a draft row counts only once Buffer holds it
        .filter((d) => d.status === 'queued' || (d.status === 'draft' && d.bufferPostId))
        .map((d) => ({ channelId: d.channelId, service: d.network, status: d.status as 'queued' | 'draft', when: null, note: '', postId: d.bufferPostId }))),
      () => live && setFetched([]),
    )
    return () => {
      live = false
    }
  }, [given, ok, memberId, token])

  if (error) return <Dead code={error.code} />
  if (view?.foreign) return <Foreign releaseUrl={view.launch.releaseUrl} />

  const outcomes = given?.length ? given : fetched
  const title = outcomes ? headline(outcomes) : ''
  const postId = outcomes?.find((o) => o.postId)?.postId
  const allDrafts = outcomes?.every((o) => o.status === 'draft')
  const q = encodeURIComponent

  async function toggle(next: boolean) {
    if (!memberId) return
    setSaved(next)
    setSaveFailed(false)
    try {
      await remember(memberId, token, next)
    } catch {
      setSaved(!next)
      setSaveFailed(true)
    }
  }

  return (
    <Shell width="person">
      <Wordmark />
      {!view || !outcomes ? (
        <Skeleton lines={3} />
      ) : !title ? (
        <>
          <h1 className="crew-title">Nothing queued yet</h1>
          <p><Link to={`/s/${q(token)}/approve`}>Back to your drafts</Link></p>
        </>
      ) : (
        <>
          <h1 className="crew-title">{title}</h1>
          <p>
            <a className="btn btn-secondary" href={postId ? `${BUFFER}/posts/${q(postId)}` : BUFFER} target="_blank" rel="noopener noreferrer">
              Open in Buffer
            </a>
          </p>
          <ul className="crew-results">
            {outcomes.map((o) => (
              <Row
                key={o.channelId}
                name={serviceName(o.service)}
                state={o.status === 'queued' ? (o.when ? `queued ${when(o.when)}` : 'queued') : o.status === 'draft' ? 'saved as draft' : o.note || 'not added'}
                tone={o.status === 'queued' ? 'ok' : o.status === 'draft' ? 'muted' : 'err'}
              />
            ))}
          </ul>
          {view.launch.launchAt && !allDrafts && <p className="crew-caption">{caption(view.launch.launchAt, view.launch.windowMinutes)}</p>}
        </>
      )}
      {memberId && <Board memberId={memberId} token={token} shareUrl={view!.launch.shortUrl || view!.launch.releaseUrl} />}
      {memberId && (
        <div className="crew-remember">
          <label className="crew-radio">
            <input type="checkbox" checked={saved} onChange={(e) => toggle(e.target.checked)} />
            Remember me for future launches
          </label>
          {saveFailed && <p className="crew-note">couldn't save, try again</p>}
        </div>
      )}
      <Footer />
    </Shell>
  )
}
