import type { ReactNode } from 'react'
import { NavLink } from 'react-router'
import NavPill from '../components/NavPill'
import SchemeSwitch from '../components/SchemeSwitch'
import Shell from '../components/Shell'
import './captain.css'

// Skip link, floating nav pill (brand, nav, account menu), the one <main>, and the appearance switch.
export default function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="cap-page">
      <a className="cap-skip" href="#main">
        Skip to main content
      </a>
      <div className="cap-top">
        <NavPill
          home="/captain"
          links={
            <>
              <li>
                <NavLink to="/captain" end>
                  Launches
                </NavLink>
              </li>
              <li>
                <NavLink to="/captain/repos">Repos</NavLink>
              </li>
            </>
          }
          end={
            <details className="cap-account">
              <summary>Account</summary>
              <div className="cap-account-menu">
                <a href="/settings">Settings</a>
                <a href="/auth/logout">Sign out</a>
              </div>
            </details>
          }
        />
      </div>
      <Shell width="captain" id="main">
        {children}
      </Shell>
      <footer className="cap-footer">
        <SchemeSwitch />
      </footer>
    </div>
  )
}
