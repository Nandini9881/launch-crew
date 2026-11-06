import type { ReactNode } from 'react'
import NavPill from './components/NavPill'
import Pass from './components/Pass'
import Plane from './components/Plane'
import SchemeSwitch from './components/SchemeSwitch'
import Wordmark from './components/Wordmark'
import './home.css'

const SIGN_IN = '/auth/buffer/start?return_to=/captain'

function Tick({ warn }: { warn?: boolean }) {
  return (
    <span className={warn ? 'home-tick home-tick-warn' : 'home-tick'} aria-hidden="true">
      <svg viewBox="0 0 12 12">{warn ? <path d="M6 2.5v4.2M6 9.2v.1" /> : <path d="M2 6.3 L5 9 L10 3" />}</svg>
    </span>
  )
}

function Section({ id, className, children }: { id?: string; className?: string; children: ReactNode }) {
  return (
    <section id={id} className={['home-section', className].filter(Boolean).join(' ')}>
      {children}
    </section>
  )
}

const PROBLEMS = [
  ['One account posts.', 'The notes go out once, from the org, and that is the whole launch.'],
  ['Everyone else reposts it.', 'The same sentence, in nine feeds, in nine voices that all sound like one.'],
  ['Or they stay quiet.', 'Writing your own take means a blank box and ten minutes nobody has on release day.'],
]

const STEPS = [
  ['Release detected', 'A GitHub release lands. The crew comes from the merged PRs, or you start from a meetup, a talk, or a manual launch.'],
  ['Drafts built in each voice', "One draft per person, written from that person's own past posts. Their role changes how it opens."],
  ['Crew approves on their phone', 'One link, one tap. They can edit it first, or not send it at all.'],
  ["Everyone's queue fills", "Each approved post goes into that person's own Buffer queue, spread across the launch window."],
]

const CREW: [string, string, string, 'ok' | 'wip' | 'new', string][] = [
  ['MK', 'Maya Kapoor', '#482', 'ok', 'Approved'],
  ['SO', 'Sam Okonkwo', '#468', 'ok', 'Approved'],
  ['HB', 'Hannah Bergström-Lindqvist', '#489', 'wip', 'Drafting'],
  ['RM', 'Rafael Montenegro-Silva', '#492', 'new', 'Invited'],
]

const CONTROL = [
  'Launch Crew writes one post into your own queue, and nothing else.',
  'It never reads or touches anything else in that queue.',
  'Disconnect whenever you like. The post stays yours.',
  'Nothing is sent without your tap, and you can edit the draft first.',
]

const PREFLIGHT = [
  'Contributor card images reachable',
  'Daily posting limits respected',
  "Each channel's network rules checked",
  'LinkedIn link goes in the first comment',
  'No overlapping drafts in any queue',
  'Buffer rate headroom',
]

// Public root: what this is, how it works, and the way in.
export default function Home() {
  return (
    <main className="home">
      <div className="home-wrap">
        <NavPill
          links={
            <>
              <li><a href="#why">Why</a></li>
              <li><a href="#how">How it works</a></li>
              <li><a href="#privacy">Privacy</a></li>
            </>
          }
          end={<a className="btn btn-primary" href={SIGN_IN}>Sign in with Buffer</a>}
        />

        <header className="home-hero">
          <div className="home-hero-copy">
            <span className="eyebrow">For release day</span>
            <h1 className="home-title">One release. Nine people. Nine posts, all theirs.</h1>
            <p className="home-lede">
              Each person who helped ship it gets a draft written in their own voice, from their own past posts, and approves it into their own Buffer queue.
            </p>
            <div className="home-cta">
              <a className="btn btn-primary" href={SIGN_IN}>
                <Plane className="home-cta-plane" />
                Start a launch
              </a>
              <a className="btn btn-secondary" href="#how">See how it works</a>
            </div>
            <p className="home-assure">Nothing posts until each person taps approve. Sign-in is with Buffer.</p>
          </div>

          <div className="home-stack" aria-hidden="true">
            <div className="home-stack-cards">
              <div className="home-stack-back home-stack-back-a">
                <Pass eyebrow="v2.3.0" name="Wei Chen" meta="@weichen · Reviewer" stub={{ label: 'Crew', value: '7', date: '3 Nov' }} />
              </div>
              <div className="home-stack-back home-stack-back-b">
                <Pass eyebrow="Meetup #12" name="Priya Nair" meta="@priyan · Speaker" stub={{ label: 'Crew', value: '38', date: '6 Nov' }} />
              </div>
              <div className="home-stack-front">
                <Pass
                  eyebrow="v2.4.0 · PR #482"
                  name="Maya Kapoor"
                  meta="@mayakp · Contributor"
                  footer="shubham-tomar/launch-crew"
                  stub={{ label: 'Crew', value: '9', date: '6 Nov' }}
                />
              </div>
            </div>
            <p className="home-stack-note">Every contributor gets a card like this, 1200 × 630, attached to their post.</p>
          </div>
        </header>

        <Section id="why">
          <h2 className="home-h2">Release day usually looks like this</h2>
          <ol className="home-problems">
            {PROBLEMS.map(([lead, body], i) => (
              <li className="panel home-problem" key={lead}>
                <span className="home-num">{i + 1}</span>
                <h3 className="home-h3">{lead}</h3>
                <p>{body}</p>
              </li>
            ))}
          </ol>
        </Section>

        <Section id="how">
          <h2 className="home-h2">How it works</h2>
          <ol className="home-steps">
            {STEPS.map(([title, body], i) => (
              <li className="panel home-step" key={title}>
                <span className="eyebrow">Step {['one', 'two', 'three', 'four'][i]}</span>
                <div className="home-ill" aria-hidden="true">
                  {i === 0 && (
                    <>
                      <span className="home-ill-chip">v2.4.0</span>
                      <span className="home-ill-line" />
                      <span className="home-ill-line home-ill-short" />
                    </>
                  )}
                  {i === 1 && (
                    <>
                      <span className="home-ill-line" />
                      <span className="home-ill-line" />
                      <span className="home-ill-line home-ill-short" />
                    </>
                  )}
                  {i === 2 && <span className="home-ill-btn">Add to my queue</span>}
                  {i === 3 && (
                    <>
                      <span className="home-ill-row" />
                      <span className="home-ill-row" />
                      <span className="home-ill-row" />
                    </>
                  )}
                </div>
                <h3 className="home-h3">{title}</h3>
                <p>{body}</p>
              </li>
            ))}
          </ol>
        </Section>

        <Section>
          <h2 className="home-h2">Two screens, two people</h2>
          <div className="home-two">
            <figure className="home-fig">
              <div className="panel home-roster" aria-hidden="true">
                <div className="home-roster-head">
                  <span className="home-h3">The crew · 9</span>
                  <span className="home-pill-ok">6 approved</span>
                </div>
                <ul>
                  {CREW.map(([ini, name, pr, tone, state]) => (
                    <li key={name} className="home-roster-row">
                      <span className="home-avatar">{ini}</span>
                      <span className="home-roster-name">{name}</span>
                      <span className="home-roster-pr">{pr}</span>
                      <span className={`home-state home-state-${tone}`}>{state}</span>
                    </li>
                  ))}
                </ul>
              </div>
              <figcaption>
                The captain sees the whole crew on one page, invited, drafting or approved, and can only launch once preflight passes.
              </figcaption>
            </figure>
            <figure className="home-fig">
              <div className="panel home-phone" aria-hidden="true">
                <span className="home-pill-ok">You are credited</span>
                <span className="home-h3">Split-pane resize with the keyboard</span>
                <div className="home-draft">
                  <p>keyboard split-pane resize shipped in v2.4.0 — alt+arrow nudges a divider, escape puts it back.</p>
                  <span className="home-count">128 / 280</span>
                </div>
                <div className="home-chips">
                  <span className="home-chip home-chip-on">X</span>
                  <span className="home-chip home-chip-on">LinkedIn</span>
                  <span className="home-chip">Threads</span>
                </div>
                <span className="home-ill-btn">Add to my queue</span>
              </div>
              <figcaption>The contributor opens one link, reads a draft that already sounds like them, and taps once.</figcaption>
            </figure>
          </div>
        </Section>

        <Section>
          <div className="home-voice-head">
            <h2 className="home-h2">Written in their voice</h2>
            <p>Maya writes short and lowercase. So does her draft.</p>
          </div>
          <div className="home-voice">
            <div className="panel">
              <span className="home-label">Her last two posts</span>
              <p className="home-quote">spent the afternoon making the focus ring survive a drag. tiny thing, feels so much better.</p>
              <p className="home-quote">shipped my first PR to a repo i actually use every day. weirdly emotional about it.</p>
            </div>
            <div className="home-voice-arrow" aria-hidden="true">
              <Plane deg={90} className="home-voice-plane" />
            </div>
            <div className="panel home-voice-out">
              <span className="home-label">The draft it produced</span>
              <p className="home-quote home-quote-hero">
                keyboard split-pane resize shipped in v2.4.0 — alt+arrow nudges a divider, escape puts it back. my first PR on this repo!
              </p>
              <div className="home-voice-meta">
                <span className="home-count">128 / 280</span>
                <span>Hers to edit, or not to send</span>
              </div>
            </div>
          </div>
        </Section>

        <Section id="privacy" className="home-control">
          <div className="home-control-main">
            <h2 className="home-h2">You stay in control</h2>
            <ul className="home-checks">
              {CONTROL.map((line) => (
                <li key={line}>
                  <Tick />
                  <span>{line}</span>
                </li>
              ))}
            </ul>
            <p className="home-quiet">
              <a href="/settings">Opt out or delete your data</a>
            </p>
          </div>
          <div className="panel home-preflight">
            <span className="home-label">Preflight, before anything goes out</span>
            <ul className="home-checks">
              {PREFLIGHT.map((line) => (
                <li key={line}>
                  <Tick />
                  <span>{line}</span>
                </li>
              ))}
              <li>
                <Tick warn />
                <span>Launch time is in the past. Pick a later moment.</span>
              </li>
            </ul>
            <p className="home-quiet">Seven checks. The launch button stays off until they pass.</p>
          </div>
        </Section>

        <Section className="home-final">
          <div className="home-final-copy">
            <h2 className="home-h2 home-final-title">Run your next release as a crew.</h2>
            <p>Nothing posts until each person taps approve.</p>
          </div>
          <a className="btn btn-secondary home-final-btn" href={SIGN_IN}>Sign in with Buffer</a>
        </Section>

        <footer className="home-footer">
          <Wordmark />
          <ul>
            <li><a href="#privacy">Privacy</a></li>
            <li><a href="https://github.com/shubham-tomar/launch-crew">GitHub</a></li>
          </ul>
          <span>Built for Buffer Commit 2026</span>
          <SchemeSwitch />
        </footer>
      </div>
    </main>
  )
}
