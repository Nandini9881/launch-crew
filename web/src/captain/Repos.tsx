import { useEffect, useState } from 'react'
import { listRepos, patchRepo, type ApiError, type Repo } from '../api'
import Alert from '../components/Alert'
import Skeleton from '../components/Skeleton'
import Frame from './Frame'
import { asError } from './format'

export default function Repos() {
  const [repos, setRepos] = useState<Repo[]>()
  const [error, setError] = useState<ApiError>()

  useEffect(() => {
    let live = true
    listRepos()
      .then((r) => live && setRepos(r))
      .catch((e) => live && setError(asError(e)))
    return () => {
      live = false
    }
  }, [])

  async function toggle(r: Repo, p: { enabled?: boolean; allowPrerelease?: boolean }) {
    setError(undefined)
    try {
      const next = await patchRepo(r.id, p)
      setRepos((rs) => rs?.map((x) => (x.id === next.id ? next : x)))
    } catch (e) {
      setError(asError(e))
    }
  }

  return (
    <Frame>
      <h1 className="cap-title">Repos</h1>
      <p className="cap-muted">Launch Crew watches releases on repos you enable. Drafts and prereleases are skipped unless you allow them.</p>
      <p>
        <a className="btn btn-primary" href="/github/install/start">
          Install the GitHub App
        </a>
      </p>
      {error && <Alert tone="error">{error.message}</Alert>}
      {!repos ? (
        !error && (
          <div aria-busy="true">
            <Skeleton lines={3} />
          </div>
        )
      ) : repos.length === 0 ? (
        <p className="cap-muted">No repos yet. Install the app to pick some.</p>
      ) : (
        <ul className="cap-list">
          {repos.map((r) => (
            <li key={r.id} className="cap-repo">
              <span className="cap-mono">{r.fullName}</span>
              <label className="cap-tick">
                <input
                  type="checkbox"
                  checked={r.enabled}
                  aria-label={`${r.fullName}: watch releases`}
                  onChange={(e) => toggle(r, { enabled: e.target.checked })}
                />
                Watch releases
              </label>
              <label className="cap-tick">
                <input
                  type="checkbox"
                  checked={r.allowPrerelease}
                  aria-label={`${r.fullName}: include prereleases`}
                  onChange={(e) => toggle(r, { allowPrerelease: e.target.checked })}
                />
                Include prereleases
              </label>
            </li>
          ))}
        </ul>
      )}
    </Frame>
  )
}
