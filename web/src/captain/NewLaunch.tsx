import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { createLaunch, type NewLaunch as Body, type Role } from '../api'
import Alert from '../components/Alert'
import Button from '../components/Button'
import Pass from '../components/Pass'
import Frame from './Frame'
import { asError, fromLocalInput } from './format'

const DAY = (v: string) => (v ? new Date(v).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }) : 'Date TBD')

const MAX_ROLES = 4
const MAX_NAME = 24

export default function NewLaunch() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [summary, setSummary] = useState('')
  const [at, setAt] = useState('')
  const [win, setWin] = useState('90')
  const [roles, setRoles] = useState<Role[]>([])
  const [hero, setHero] = useState('')
  const [heroError, setHeroError] = useState<string>()
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const setRole = (i: number, patch: Partial<Role>) =>
    setRoles((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)))

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError(undefined)
    setHeroError(undefined)
    const url = hero.trim()
    if (url && !/^https:\/\/\S+$/i.test(url)) {
      setHeroError('Use an https:// image URL.')
      return
    }
    const body: Body = { title: title.trim() }
    if (summary.trim()) body.summary = summary.trim()
    if (at) body.launchAt = fromLocalInput(at)
    const n = Number(win)
    if (Number.isInteger(n) && n > 0) body.windowMinutes = n
    const named = roles
      .filter((r) => r.name.trim())
      .slice(0, MAX_ROLES)
      .map((r) => ({ name: r.name.trim().slice(0, MAX_NAME), hint: r.hint.trim() }))
    if (named.length) body.roles = named
    if (url) body.heroUrl = url
    setBusy(true)
    try {
      const d = await createLaunch(body)
      navigate(`/captain/launches/${d.id}`)
    } catch (err) {
      const ex = asError(err)
      if (ex.code === 'hero_refused' || ex.code === 'hero_not_image') setHeroError(ex.message)
      else setError(ex.message)
      setBusy(false)
    }
  }

  return (
    <Frame>
      <h1 className="cap-title">New launch</h1>
      <div className="cap-new">
      <form className="cap-form panel" onSubmit={submit}>
        <p className="cap-lede">Launch Crew writes one draft per person and holds every one of them until that person approves it.</p>
        <label className="cap-field">
          Title
          <input className="field" type="text" required maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label className="cap-field">
          Summary
          <textarea className="field" rows={3} value={summary} onChange={(e) => setSummary(e.target.value)} />
        </label>
        <label className="cap-field">
          Launch time
          <input className="field" type="datetime-local" value={at} onChange={(e) => setAt(e.target.value)} />
        </label>
        <label className="cap-field">
          Window (minutes)
          <input className="field" type="number" min={1} value={win} onChange={(e) => setWin(e.target.value)} />
        </label>
        <fieldset className="cap-fieldset">
          <legend>Roles</legend>
          <p className="cap-muted">Leave empty for Attendee, Speaker and Organiser. Up to four.</p>
          {roles.map((r, i) => (
            <div key={i} className="cap-role">
              <label className="cap-field">
                <span className="visually-hidden">Role {i + 1} name</span>
                <input
                  className="field"
                  type="text"
                  placeholder="Name"
                  aria-label={`Role ${i + 1} name`}
                  maxLength={MAX_NAME}
                  value={r.name}
                  onChange={(e) => setRole(i, { name: e.target.value })}
                />
              </label>
              <label className="cap-field">
                <span className="visually-hidden">Role {i + 1} hint</span>
                <input
                  className="field"
                  type="text"
                  placeholder="Hint, e.g. spoke about"
                  aria-label={`Role ${i + 1} hint`}
                  maxLength={60}
                  value={r.hint}
                  onChange={(e) => setRole(i, { hint: e.target.value })}
                />
              </label>
              <Button variant="ghost" onClick={() => setRoles((rs) => rs.filter((_, j) => j !== i))}>
                Remove<span className="visually-hidden"> role {i + 1}</span>
              </Button>
            </div>
          ))}
          <Button
            variant="secondary"
            disabled={roles.length >= MAX_ROLES}
            onClick={() => setRoles((rs) => [...rs, { name: '', hint: '' }])}
          >
            Add a role
          </Button>
        </fieldset>
        <label className="cap-field">
          Hero image URL (optional)
          <input
            className="field"
            type="text"
            inputMode="url"
            value={hero}
            aria-invalid={heroError ? true : undefined}
            aria-describedby={heroError ? 'cap-hero-error' : undefined}
            onChange={(e) => setHero(e.target.value)}
          />
        </label>
        {heroError && <span id="cap-hero-error" role="alert" className="cap-error">{heroError}</span>}
        {error && <Alert tone="error">{error}</Alert>}
        <Button variant="primary" type="submit" busy={busy} disabled={busy}>
          Create launch
        </Button>
      </form>
      <aside className="cap-preview" aria-label="Live preview">
        <span className="cap-kicker">Contributor card</span>
        <Pass
          eyebrow={title.trim() || 'Your launch'}
          name="A contributor"
          meta={`@handle · ${roles.find((r) => r.name.trim())?.name.trim() || 'Contributor'}`}
          stub={{ label: 'Pass', value: '#1', date: DAY(at) }}
        />
        <p className="cap-muted">A sample. Each person gets their own, rebuilt when the drafts go out.</p>
        {summary.trim() && (
          <div className="panel">
            <span className="cap-kicker">What the crew reads</span>
            <p className="cap-preview-title">{title.trim() || 'Your launch'}</p>
            <p className="cap-muted">{summary.trim()}</p>
          </div>
        )}
      </aside>
      </div>
    </Frame>
  )
}
