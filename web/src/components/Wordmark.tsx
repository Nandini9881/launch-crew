import Plane from './Plane'
import './components.css'

export default function Wordmark() {
  return (
    <span className="wordmark">
      <span className="wordmark-mark" aria-hidden="true">
        <Plane />
      </span>
      Launch Crew
    </span>
  )
}
