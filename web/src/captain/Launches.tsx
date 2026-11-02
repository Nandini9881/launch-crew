import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { listLaunches, type ApiError, type LaunchSummary } from '../api'
import Alert from '../components/Alert'
import Skeleton from '../components/Skeleton'
import Frame from './Frame'
import { asError, stamp } from './format'

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
      <h1 className="cap-title">Launches</h1>
      {error ? (
        <Alert tone="error">{error.message}</Alert>
      ) : !items ? (
        <div aria-busy="true">
          <Skeleton lines={4} />
        </div>
      ) : items.length === 0 ? (
        <div className="cap-empty">
          <p>Nothing launched yet. Tag a release, or start one by hand.</p>
          <Actions />
        </div>
      ) : (
        <>
          <Actions />
          <ul className="cap-list">
            {items.map((l) => (
              <li key={l.id} className="cap-launch">
                <Link to={`/captain/launches/${l.id}`}>{l.title}</Link>
                <span className="cap-muted">
                  {l.status}
                  {l.releaseTag && <> · <span className="cap-mono">{l.releaseTag}</span></>}
                  {l.launchAt && ` · ${stamp(l.launchAt)}`}
                  {l.status === 'launched' && ` · ${l.openCount} scanned`}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </Frame>
  )
}
