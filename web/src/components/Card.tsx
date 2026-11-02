import './components.css'

type Props = {
  project: string
  version: string
  title: string
  image?: { url: string; alt: string } | null
}

// Share-card preview. Owns the page <h1>: visible in the CSS card, hidden when the image carries the title.
export default function Card({ project, version, title, image }: Props) {
  if (image) {
    return (
      <>
        <h1 className="visually-hidden">{title}</h1>
        <img className="card card-img" src={image.url} alt={image.alt} />
      </>
    )
  }
  return (
    <div className="card">
      <div className="card-body">
        <span className="card-meta">
          {project} {version}
        </span>
        <h1 className="card-title">{title}</h1>
      </div>
    </div>
  )
}
