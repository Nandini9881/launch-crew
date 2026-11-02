import { Link, useParams } from 'react-router'
import Card from '../components/Card'
import Shell from '../components/Shell'
import Skeleton from '../components/Skeleton'
import Wordmark from '../components/Wordmark'
import { Dead, Foreign, Footer, useInvite } from './shared'

export default function Invite() {
  const token = useParams().token ?? ''
  const { view, error } = useInvite(token)

  if (error) return <Dead code={error.code} />
  if (view?.foreign) return <Foreign releaseUrl={view.launch.releaseUrl} />

  const q = encodeURIComponent
  return (
    <Shell width="person">
      <div aria-busy={!view || undefined}>
        <Wordmark />
        {view ? (
          <Card
            project={view.launch.project}
            version={view.launch.version}
            title={view.launch.title}
            image={view.card}
          />
        ) : (
          <Skeleton lines={4} />
        )}
        {view ? <p className="crew-preview">{view.preview}</p> : <Skeleton lines={3} />}
        <p className="crew-caption">
          adds drafts to your queue only · nothing posts without your tap · disconnect any time
        </p>
        {view && (
          <div className="crew-actions">
            {view.needsVerify ? (
              <a className="btn btn-primary" href={`/github/verify/start?crew=${q(view.member.id)}&t=${q(token)}`}>
                Verify it's you with GitHub
              </a>
            ) : (
              <a className="btn btn-primary" href={`/auth/buffer/start?return_to=/s/${q(token)}/approve`}>
                Connect Buffer &amp; share
              </a>
            )}
            <Link className="btn btn-ghost" to={`/s/${q(token)}/fallback`}>
              I don't use Buffer
            </Link>
          </div>
        )}
      </div>
      <Footer />
    </Shell>
  )
}
