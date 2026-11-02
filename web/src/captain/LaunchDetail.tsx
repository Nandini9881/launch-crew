import { useState } from 'react'
import { useParams } from 'react-router'
import { pressLaunch, retryJob, type Patch } from '../api'
import Alert from '../components/Alert'
import Button from '../components/Button'
import Skeleton from '../components/Skeleton'
import Crew from './Crew'
import Frame from './Frame'
import { asError } from './format'
import Preflight from './Preflight'
import Review from './Review'
import Timing from './Timing'
import { useLaunch } from './useLaunch'

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
          <Skeleton lines={5} />
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
      {notice && <Alert tone="error">{notice}</Alert>}
      <div className="cap-grid">
        <Review detail={detail} pending={pending} onHidden={(hiddenPrs) => save({ hiddenPrs })} />
        <div className="cap-col">
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

          <Preflight
            checks={detail.checks}
            launched={launched}
            busy={busy}
            error={launchError}
            onLaunch={launch}
          />
        </div>
      </div>
    </Frame>
  )
}
