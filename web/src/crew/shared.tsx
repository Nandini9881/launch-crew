import { useEffect, useState, type ReactNode } from 'react'
import { useSearchParams } from 'react-router'
import { ApiError, getInvite, type View } from '../api'
import SchemeSwitch from '../components/SchemeSwitch'
import Shell from '../components/Shell'
import Wordmark from '../components/Wordmark'
import './crew.css'

const DEAD: Record<string, { title: string; why: string }> = {
  expired: { title: 'This link has expired', why: 'Invite links last 30 days. Ask the captain for a fresh one.' },
  not_found: { title: "This link isn't valid", why: 'It may have been mistyped or revoked. Ask the captain to resend it.' },
  crew_full: { title: 'This crew is full', why: 'The captain can raise the cap in settings.' },
  rate_limited: { title: 'Too many opens from here, try again in a minute', why: 'This link was opened many times in a short while.' },
}

// Every contributor screen: wordmark and version on the ground, one white panel, optional footer below.
export function CrewFrame({ version, footer, children }: { version?: string; footer?: boolean; children: ReactNode }) {
  return (
    <Shell width="person">
      <div className="crew-top">
        <Wordmark />
        {version && <span className="crew-version">{version}</span>}
      </div>
      <div className="crew-panel">{children}</div>
      {footer && <Footer />}
    </Shell>
  )
}

export function Dead({ code }: { code: string }) {
  const d = DEAD[code] ?? { title: 'Something went wrong', why: 'Try again in a moment.' }
  return (
    <CrewFrame>
      <h1 className="crew-title">{d.title}</h1>
      <p className="crew-lede">{d.why}</p>
    </CrewFrame>
  )
}

export function Footer() {
  return (
    <footer className="crew-footer">
      <SchemeSwitch />
      <a href="/settings">Settings</a>
      <span>Built on the Buffer API</span>
    </footer>
  )
}

export type Loaded = { view?: View; error?: ApiError }

// Loads the invite View; foreign=1 on the page URL is forwarded to the API.
export function useInvite(token: string): Loaded {
  const [params] = useSearchParams()
  const foreign = params.get('foreign') === '1'
  const [state, setState] = useState<Loaded>({})
  useEffect(() => {
    let live = true
    getInvite(token, { foreign })
      .then((view) => live && setState({ view }))
      .catch((e) => live && setState({ error: e instanceof ApiError ? e : new ApiError(0, 'error', String(e)) }))
    return () => {
      live = false
    }
  }, [token, foreign])
  return state
}

export function Foreign({ releaseUrl }: { releaseUrl: string }) {
  return (
    <CrewFrame>
      <h1 className="crew-title">This link was made for someone else</h1>
      <p className="crew-lede">Invite links are personal. Ask the captain for yours, or share the launch with your own link.</p>
      <p><a href={releaseUrl}>See the release</a></p>
    </CrewFrame>
  )
}
