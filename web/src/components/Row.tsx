import './components.css'

type Props = { name: string; meta?: string; state: string; tone?: 'ok' | 'err' | 'muted'; fix?: string }

export default function Row({ name, meta, state, tone = 'muted', fix }: Props) {
  return (
    <li className={`row row-${tone}`}>
      <div className="row-main">
        {meta ? (
          <span className="row-name">
            <span>{name}</span>
            <span className="row-meta">{meta}</span>
          </span>
        ) : (
          <span className="row-name">{name}</span>
        )}
        <span className="row-state">{state}</span>
      </div>
      {fix && <p className="row-fix">{fix}</p>}
    </li>
  )
}
