import { useEffect, useRef, useState } from 'react'
import './components.css'

type Props = {
  id: string
  serverText: string
  fromModel: boolean
  waiting: boolean // voiced version may still arrive
  limit?: number
  countFirst?: string // first thread segment, counted until the person edits
  readOnly?: boolean
  onText: (text: string) => void
  onSave: (text: string) => void
}

const reduced = () => typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

// One editable draft. A newer server text replaces it only while the person is neither
// typing in it nor has edited it; a swap waits for blur.
export default function DraftBox({ id, serverText, fromModel, waiting, limit, countFirst, readOnly, onText, onSave }: Props) {
  const [value, setValue] = useState(serverText)
  const [model, setModel] = useState(fromModel)
  const [reveal, setReveal] = useState(false)
  const area = useRef<HTMLTextAreaElement>(null)
  const cur = useRef(serverText)
  const edited = useRef(false)
  const saved = useRef(serverText)
  const pending = useRef<{ text: string; model: boolean }>(undefined)
  const first = useRef(true)

  function apply(text: string, fromM: boolean) {
    pending.current = undefined
    saved.current = text
    setModel(fromM)
    if (cur.current !== text && !reduced()) setReveal(true)
    cur.current = text
    setValue(text)
    onText(text)
  }

  useEffect(() => {
    if (first.current) {
      first.current = false
      onText(serverText)
      return
    }
    if (edited.current) return
    if (document.activeElement === area.current) pending.current = { text: serverText, model: fromModel }
    else apply(serverText, fromModel)
    // intentionally keyed on the server values only; apply/onText are stable enough
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverText, fromModel])

  useEffect(() => {
    if (!reveal) return
    const t = window.setTimeout(() => setReveal(false), 240)
    return () => window.clearTimeout(t)
  }, [reveal, value])

  function blur() {
    if (!edited.current && pending.current) apply(pending.current.text, pending.current.model)
    else if (value !== saved.current) {
      saved.current = value
      onSave(value)
    }
  }

  const n = edited.current || countFirst === undefined ? value.length : countFirst.length
  const near = limit !== undefined && n >= limit - 20
  const caption = model ? 'written how you write' : waiting ? 'a version in your voice is on its way' : ''

  return (
    <div className={reveal ? 'draftbox draft-reveal' : 'draftbox'}>
      <label className="visually-hidden" htmlFor={`draft-${id}`}>Your post</label>
      <textarea
        id={`draft-${id}`}
        ref={area}
        className="crew-draft"
        value={value}
        readOnly={readOnly}
        onChange={(e) => {
          edited.current = true
          cur.current = e.target.value
          setValue(e.target.value)
          onText(e.target.value)
        }}
        onBlur={blur}
      />
      <p className="crew-note" aria-live="polite">
        {caption}
        {near && ` ${caption ? '· ' : ''}${n} of ${limit} characters`}
      </p>
    </div>
  )
}
