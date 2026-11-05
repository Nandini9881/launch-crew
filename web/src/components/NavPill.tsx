import type { ReactNode } from 'react'
import Wordmark from './Wordmark'
import './components.css'

type Props = {
  /** Where the logo links to. */
  home?: string
  /** Navigation links, already rendered as <li><a/></li> items. */
  links?: ReactNode
  /** Right-hand side: account chip, sign-in button. */
  end?: ReactNode
}

// The floating white bar at the top of every page after the landing hero.
export default function NavPill({ home = '/', links, end }: Props) {
  return (
    <header className="navpill">
      <a className="navpill-home" href={home}>
        <Wordmark />
      </a>
      {links && (
        <nav className="navpill-nav" aria-label="Main">
          <ul>{links}</ul>
        </nav>
      )}
      {end && <div className="navpill-end">{end}</div>}
    </header>
  )
}
