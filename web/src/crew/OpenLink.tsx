import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError, chooseRole, openLink, type LaunchView } from '../api'
import Alert from '../components/Alert'
import Button from '../components/Button'
import Card from '../components/Card'
import ChoiceRow from '../components/ChoiceRow'
import Shell from '../components/Shell'
import Skeleton from '../components/Skeleton'
import Wordmark from '../components/Wordmark'
import { Dead } from './shared'
import './crew.css'

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, 'error', String(e)))

export default function OpenLink() {
  const openToken = useParams().openToken ?? ''
  const navigate = useNavigate()
  const [view, setView] = useState<LaunchView>()
  const [error, setError] = useState<ApiError>()
  const [pick, setPick] = useState(0) // roles.length is the Other row
  const [talk, setTalk] = useState('')
  const [other, setOther] = useState('')
  const [busy, setBusy] = useState(false)
  const [limited, setLimited] = useState(false)
  const [missing, setMissing] = useState(false)
  const sending = useRef(false)

  useEffect(() => {
    let live = true
    openLink(openToken)
      .then((v) => {
        if (!live) return
        setView(v)
        const i = v.roles.findIndex((r) => r.name === 'Attendee')
        setPick(i >= 0 ? i : 0)
      })
      .catch((e) => live && setError(asError(e)))
    return () => {
      live = false
    }
  }, [openToken])

  if (error) return <Dead code={error.code} />

  const roles = view?.roles ?? []
  const isOther = pick === roles.length
  const isSpeaker = !isOther && roles[pick]?.name === 'Speaker'

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (sending.current || !view) return
    if (isOther && !other.trim()) {
      setMissing(true)
      return
    }
    setMissing(false)
    sending.current = true
    setBusy(true)
    setLimited(false)
    try {
      const { token } = await chooseRole(openToken, {
        role: isOther ? 'Other' : roles[pick].name,
        hint: isOther ? other.trim() : '',
        talkTitle: isSpeaker ? talk.trim() : '',
        other: isOther,
      })
      navigate(`/s/${encodeURIComponent(token)}`)
    } catch (err) {
      const ex = asError(err)
      if (ex.status === 429) setLimited(true)
      else setError(ex)
      sending.current = false
      setBusy(false)
    }
  }

  const image = view?.card?.url ? view.card : null
  return (
    <Shell width="person">
      <div aria-busy={!view || undefined}>
        <Wordmark />
        {view ? <Card project="Event" version="" title={view.title} image={image} /> : <Skeleton lines={4} />}
        <form onSubmit={submit}>
          <fieldset className="crew-fieldset">
            <legend>What was your part?</legend>
            {view ? (
              <div className="crew-choices">
                {roles.map((r, i) => (
                  <ChoiceRow
                    key={r.name}
                    name="role"
                    value={r.name}
                    label={r.name}
                    hint={r.hint}
                    checked={pick === i}
                    onChange={() => setPick(i)}
                  />
                ))}
                <ChoiceRow name="role" value="Other" label="Other" checked={isOther} onChange={() => setPick(roles.length)} />
              </div>
            ) : (
              <Skeleton lines={4} />
            )}
          </fieldset>
          {isSpeaker && (
            <label className="crew-field">
              Talk title
              <input type="text" maxLength={120} value={talk} onChange={(e) => setTalk(e.target.value)} />
            </label>
          )}
          {isOther && (
            <label className="crew-field">
              What was your part?
              <input type="text" maxLength={40} required value={other} onChange={(e) => setOther(e.target.value)} />
              {missing && <span role="alert">Say what your part was.</span>}
            </label>
          )}
          {limited && <Alert tone="warning" role="alert">Too many opens from here, try again in a minute.</Alert>}
          <div className="crew-actions">
            <Button variant="primary" type="submit" busy={busy} disabled={busy}>
              Continue
            </Button>
          </div>
        </form>
      </div>
    </Shell>
  )
}
