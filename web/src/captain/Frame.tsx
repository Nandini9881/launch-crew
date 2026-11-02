import type { ReactNode } from 'react'
import { Link } from 'react-router'
import SchemeSwitch from '../components/SchemeSwitch'
import Shell from '../components/Shell'
import Wordmark from '../components/Wordmark'
import './captain.css'

// Skip link, header (wordmark, nav, account menu, scheme switch) and the one <main>.
export default function Frame({ children }: { children: ReactNode }) {
  return (
    <>
      <a className="cap-skip" href="#main">
        Skip to main content
      </a>
      <header className="cap-header">
        <Link to="/captain" className="cap-brand" aria-label="Launch Crew, all launches">
          <Wordmark />
        </Link>
        <nav className="cap-nav" aria-label="Captain">
          <Link to="/captain">Launches</Link>
          <Link to="/captain/repos">Repos</Link>
        </nav>
        <SchemeSwitch />
        <details className="cap-account">
          <summary>Account</summary>
          <div className="cap-account-menu">
            <a href="/settings">Settings</a>
            <a href="/auth/logout">Sign out</a>
          </div>
        </details>
      </header>
      <Shell width="captain" id="main">
        {children}
      </Shell>
    </>
  )
}
