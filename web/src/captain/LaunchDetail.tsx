import { useState } from 'react'
import { useParams } from 'react-router'
import { pressLaunch, retryJob, type Detail, type Patch } from '../api'
import Alert from '../components/Alert'
import Button from '../components/Button'
import Loader from '../components/Loader'
import Crew from './Crew'
import Frame from './Frame'
import { asError, stamp } from './format'
import Preflight from './Preflight'
import Review from './Review'
import Timing from './Timing'
import { useLaunch } from './useLaunch'

type Step = { name: string; note: string; state: 'done' | 'now' | 'next' }

// Where the launch is: release in, drafts built, crew joining, then the launch itself.
function Steps({ detail, launched }: { detail: Detail; launched: boolean }) {
  const built = detail.captainDrafts.length > 0
  const crewing = detail.crew.length > 0
  const raw = [
    { name: detail.source === 'github' ? 'Release detected' : 'Launch created', note: detail.releaseTag || 'by hand', done: true },
    { name: 'Drafts built', note: built ? `${detail.captainDrafts.length} of yours` : 'building', done: built || launched },
    { name: 'Crew joining', note: `${detail.counts.in} of ${detail.counts.total} in`, done: launched },
    { name: 'Launch', note: detail.launchAt ? stamp(detail.launchAt) : 'time not set', done: launched },
  ]
  const now = raw.findIndex((s) => !s.done)
  const steps: Step[] = raw.map((s, i) => ({ name: s.name, note: s.note, state: s.done ? 'done' : i === now ? 'now' : 'next' }))
  if (!crewing && now === 2) steps[2].note = 'no one yet'
  return (
    <ol className="cap-steps" aria-label="Progress">
      {steps.map((s) => (
        <li key={s.name} className={`cap-step cap-step-${s.state}`} aria-current={s.state === 'now' ? 'step' : undefined}>
          <strong>{s.name}</strong>
          <span className="cap-muted">{s.note}</span>
        </li>
      ))}
    </ol>
  )
}

export default function LaunchDetail() {
  const id = useParams().id ?? ''
  const { detail, error, reload, patch } = useLaunch(id)
  const [pending, setPending] = useState(false)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<string>()
  const [pressed, setPressed] = useState(false)
  const [launchError, setLaunchError] = useState<string>()

  async function run(fn: () => Promise<unknown>) {
    setNotice(undefined)
    setPending(true)
    try {
      await fn()
    } catch (e) {
      setNotice(asError(e).message)
    }
    setPending(false)
  }
  const save = (p: Patch) => run(() => patch(p))

  async function launch() {
    setBusy(true)
    setLaunchError(undefined)
    try {
      await pressLaunch(id)
      setPressed(true)
      await reload()
    } catch (e) {
      const ex = asError(e)
      if (ex.code === 'already_launched') await reload()
      else {
        setLaunchError(ex.message)
        if (ex.code === 'preflight_red') await reload()
      }
    }
    setBusy(false)
  }

  if (error && !detail) {
    return (
      <Frame>
        <h1>{error.status === 404 ? 'Launch not found' : 'Could not load this launch'}</h1>
        <p>{error.status === 404 ? 'It may belong to another captain.' : error.message}</p>
      </Frame>
    )
  }
  if (!detail) {
    return (
      <Frame>
        <div aria-busy="true">
          <Loader label="Loading this launch" />
        </div>
      </Frame>
    )
  }

  const launched = detail.status !== 'draft' || pressed
  const manual = detail.source === 'manual'
  return (
    <Frame>
      {detail.deadLetters.map((d) => (
        <div key={d.jobId} className="cap-dead">
          <Alert tone="error" role="alert" lead={`Step failed: ${d.kind}.`}>
            {d.lastError} <span className="cap-muted">({d.attempts} attempts)</span>{' '}
            <Button variant="secondary" disabled={pending} onClick={() => run(async () => { await retryJob(id, d.jobId); await reload() })}>
              Retry
            </Button>
          </Alert>
        </div>
      ))}
      <h1 className="cap-title">{detail.title}</h1>
      {launched && (
        <p className="cap-muted">
          Launched · {detail.counts.in} of {detail.counts.total} in · {detail.openCount} scanned
        </p>
      )}
      <Steps detail={detail} launched={launched} />
      {notice && <Alert tone="error">{notice}</Alert>}
      <div className="cap-grid">
        <Review detail={detail} pending={pending} onHidden={(hiddenPrs) => save({ hiddenPrs })} />
        <div className="cap-col">
          <Preflight
            checks={detail.checks}
            launched={launched}
            busy={busy}
            error={launchError}
            onLaunch={launch}
          />
          <Crew detail={detail} pending={pending} onTicked={(tickedCrew) => save({ tickedCrew })} />
          <Timing key={`${detail.launchAt}-${detail.windowMinutes}`} detail={detail} onSave={save} />
          {manual && detail.status === 'draft' && (
            <p className="cap-muted">Link, slide and PNG appear after Launch.</p>
          )}
          {manual && detail.showUrl && (
            <p className="cap-links">
              <a href={detail.showUrl}>Show at the event</a>
              <a href={detail.pngUrl} download>Download PNG</a>
            </p>
          )}
        </div>
      </div>
    </Frame>
  )
}
