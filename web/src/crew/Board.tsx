import { useCallback, useEffect, useRef, useState } from 'react'
import { board, type BoardView } from '../api'
import Alert from '../components/Alert'
import Button from '../components/Button'
import Row from '../components/Row'
import Skeleton from '../components/Skeleton'

const REFRESH_MS = 30_000

function describe(state: string): { text: string; tone: 'ok' | 'err' | 'muted' } {
  if (state === 'queued' || state === 'posted') return { text: state, tone: 'ok' }
  if (state === 'copied') return { text: 'shared another way', tone: 'muted' }
  const text = state.replace(/_/g, ' ')
  return { text, tone: state.includes('failed') ? 'err' : 'muted' }
}

export default function Board({ memberId, token, shareUrl }: { memberId: string; token: string; shareUrl: string }) {
  const [data, setData] = useState<BoardView>()
  const [failed, setFailed] = useState(false)
  const [copied, setCopied] = useState(false)
  const live = useRef(true)

  const load = useCallback(() => {
    board(memberId, token).then(
      (v) => live.current && (setData(v), setFailed(false)),
      () => live.current && setFailed(true),
    )
  }, [memberId, token])

  useEffect(() => {
    live.current = true
    load()
    const t = window.setTimeout(load, REFRESH_MS) // one refresh, not a loop
    return () => {
      live.current = false
      window.clearTimeout(t)
    }
  }, [load])

  async function copy() {
    if (!shareUrl) return
    try {
      await navigator.clipboard.writeText(shareUrl)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }

  const retry = (
    <Alert tone="warning">
      crew board is taking a moment{' '}
      <Button variant="secondary" onClick={load}>Retry</Button>
    </Alert>
  )

  if (!data) {
    return (
      <section className="crew-board" aria-label="Crew">
        {failed ? retry : <Skeleton lines={3} />}
      </section>
    )
  }

  return (
    <section className="crew-board" aria-label="Crew">
      <h2 className="crew-subtitle">Crew · {data.in} of {data.total} in</h2>
      {data.total <= 1 && (
        <div className="crew-actions">
          <p className="crew-note">Just you so far. Share the link and the board fills in.</p>
          {shareUrl && <Button variant="secondary" onClick={copy}>{copied ? 'Copied' : 'Copy link'}</Button>}
        </div>
      )}
      <ul className="crew-results">
        {data.rows.map((r, i) => {
          const d = describe(r.state)
          const meta = r.role && r.role !== r.name ? r.role : undefined
          return <Row key={`${r.name}-${i}`} name={r.name} meta={meta} state={d.text} tone={d.tone} />
        })}
      </ul>
      {failed && retry}
    </section>
  )
}
