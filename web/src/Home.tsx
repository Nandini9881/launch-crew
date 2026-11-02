import SchemeSwitch from './components/SchemeSwitch'
import Shell from './components/Shell'
import Wordmark from './components/Wordmark'
import './home.css'

// Public root: what this is, one way in, and the way out.
export default function Home() {
  return (
    <Shell width="person">
      <h1 className="home-title">
        <Wordmark />
      </h1>
      <p className="home-lede">
        Launch Crew turns one announcement into many people's posts, each approved by that person inside their own Buffer account.
      </p>
      <p>
        <a className="btn btn-primary" href="/auth/buffer/start?return_to=/captain">
          Sign in with Buffer
        </a>
      </p>
      <p className="home-quiet">
        <a href="/settings">Opt out or delete your data</a>
      </p>
      <footer className="home-footer">
        <SchemeSwitch />
      </footer>
    </Shell>
  )
}
