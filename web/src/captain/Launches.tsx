import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { listLaunches, type ApiError, type LaunchSummary } from '../api'
import Alert from '../components/Alert'
import Loader from '../components/Loader'
import Frame from './Frame'
import { asError, stamp, until } from './format'

const STATUS: Record<LaunchSummary['status'], string> = { draft: 'Draft', launched: 'Live', closed: 'Closed' }

function Actions() {
  return (
    <div className="cap-actions">
      <Link className="btn btn-primary" to="/captain/repos">
        Connect a repo
      </Link>
      <Link className="btn btn-secondary" to="/captain/new">
        Start one by hand
      </Link>
    </div>
  )
}

function Ghost() {
  return (
    <div className="cap-ghost" aria-hidden="true">
      <div className="cap-ghost-main">
        <span className="cap-ghost-bar" />
        <span className="cap-ghost-block" />
        <span className="cap-ghost-bar" />
      </div>
      <div className="perf perf-v cap-ghost-perf" />
      <div className="cap-ghost-side">
        <span className="cap-ghost-block" />
        <span className="cap-ghost-bar" />
      </div>
    </div>
  )
}

function LaunchCard({ l }: { l: LaunchSummary }) {
  const left = l.status === 'closed' ? '' : until(l.launchAt)
  return (
    <li className="cap-launch panel">
      <div className="cap-launch-top">
        <span className={`cap-chip cap-chip-${l.status}`}>{STATUS[l.status]}</span>
        {left && <span className="cap-count">in {left}</span>}
      </div>
      <Link className="cap-launch-title" to={`/captain/launches/${l.id}`}>
        {l.title}
      </Link>
      <span className="cap-muted">
        {l.source === 'github' ? 'GitHub release' : 'Manual launch'}
        {l.releaseTag && <> · <span className="cap-mono">{l.releaseTag}</span></>}
        {l.launchAt && ` · ${stamp(l.launchAt)} · ${l.windowMinutes} min`}
        {l.status === 'launched' && ` · ${l.openCount} scanned`}
      </span>
    </li>
  )
}

export default function Launches() {
  const [items, setItems] = useState<LaunchSummary[]>()
  const [error, setError] = useState<ApiError>()

  useEffect(() => {
    let live = true
    listLaunches()
      .then((l) => live && setItems(l))
      .catch((e) => live && setError(asError(e)))
    return () => {
      live = false
    }
  }, [])

  return (
    <Frame>
      {error ? (
        <>
          <h1 className="cap-title">Launches</h1>
          <Alert tone="error">{error.message}</Alert>
        </>
      ) : !items ? (
        <>
          <h1 className="cap-title">Launches</h1>
          <div aria-busy="true">
            <Loader label="Loading your launches" />
          </div>
        </>
      ) : items.length === 0 ? (
        <section className="cap-empty panel">
          <div className="cap-empty-copy">
            <span className="eyebrow">Board is clear</span>
            <h1 className="cap-title">No launches yet</h1>
            <p>Nothing launched yet. Tag a release, or start one by hand.</p>
            <p className="cap-muted">
              Everyone who helped gets a pass with their own post on it, waiting for their tap. Nothing posts until each person approves.
            </p>
            <Actions />
          </div>
          <Ghost />
        </section>
      ) : (
        <>
          <div className="cap-head">
            <h1 className="cap-title">Launches</h1>
            <Actions />
          </div>
          <ul className="cap-list cap-cards">
            {items.map((l) => (
              <LaunchCard key={l.id} l={l} />
            ))}
          </ul>
        </>
      )}
    </Frame>
  )
}
