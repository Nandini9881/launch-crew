import { useEffect, useState } from 'react'
import { listRepos, patchRepo, type ApiError, type Repo } from '../api'
import Alert from '../components/Alert'
import Loader from '../components/Loader'
import Frame from './Frame'
import { asError } from './format'
import './repos.css'

const INSTALL = '/github/install/start'

const plus = (
  <svg viewBox="0 0 14 14" aria-hidden="true">
    <path d="M7 2v10M2 7h10" />
  </svg>
)

const repoIcon = (
  <svg viewBox="0 0 20 20" aria-hidden="true">
    <path d="M5 3.5h8.5a1.5 1.5 0 0 1 1.5 1.5v11H6.5A1.5 1.5 0 0 1 5 14.5V3.5z" />
    <path d="M5 14.5A1.5 1.5 0 0 1 6.5 13H15" />
  </svg>
)

function Switch({ label, name, checked, onChange }: { label: string; name: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="rp-switch">
      <input type="checkbox" role="switch" checked={checked} aria-label={`${name}: ${label.toLowerCase()}`} onChange={(e) => onChange(e.target.checked)} />
      <span className="rp-track" aria-hidden="true">
        <span className="rp-thumb" />
      </span>
      <span className="rp-switch-text">{label}</span>
    </label>
  )
}

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

  const add = (
    <a className="btn btn-primary rp-add" href={INSTALL}>
      {plus}
      Add a repo
    </a>
  )

  return (
    <Frame>
      <div className="rp-head">
        <div className="rp-head-text">
          <h1 className="rp-title">Repos</h1>
          <p className="rp-lede">
            Launch Crew watches releases on the repos you add. A published release becomes a draft launch here; drafts and prereleases are
            skipped unless you allow them.
          </p>
        </div>
        {add}
      </div>
      {error && <Alert tone="error">{error.message}</Alert>}
      {!repos ? (
        !error && (
          <div aria-busy="true">
            <Loader label="Loading your repos" />
          </div>
        )
      ) : repos.length === 0 ? (
        <section className="rp-empty" aria-label="No repos yet">
          <span className="rp-icon" aria-hidden="true">
            {repoIcon}
          </span>
          <h2>No repos yet</h2>
          <p>Add a repo and every release you publish in it creates a draft launch for you to review. Nothing posts until you press Launch.</p>
          {add}
        </section>
      ) : (
        <ul className="rp-list">
          {repos.map((r) => (
            <li key={r.id} className="rp-repo">
              <span className="rp-icon" aria-hidden="true">
                {repoIcon}
              </span>
              <span className="rp-name">
                <strong>{r.fullName}</strong>
                <span>{r.enabled ? (r.allowPrerelease ? 'Watching releases and prereleases' : 'Watching releases') : 'Paused'}</span>
              </span>
              <span className="rp-switches">
                <Switch label="Watch releases" name={r.fullName} checked={r.enabled} onChange={(v) => toggle(r, { enabled: v })} />
                <Switch label="Include prereleases" name={r.fullName} checked={r.allowPrerelease} onChange={(v) => toggle(r, { allowPrerelease: v })} />
              </span>
            </li>
          ))}
        </ul>
      )}
    </Frame>
  )
}
