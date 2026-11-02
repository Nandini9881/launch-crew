import type { Ref } from 'react'
import type { Channel, Outcome } from '../api'
import Row from '../components/Row'
import { serviceName, when } from './format'

function describe(o: Outcome): { state: string; tone: 'ok' | 'err' | 'muted' } {
  if (o.status === 'queued') return { state: o.when ? `queued ${when(o.when)}` : 'queued', tone: 'ok' }
  if (o.status === 'draft') return { state: o.note || 'saved as draft', tone: 'muted' }
  // the server sends the whole sentence
  return { state: o.note || `${serviceName(o.service)} rejected`, tone: 'err' }
}

export default function Results({ outcomes, channels, listRef }: { outcomes: Outcome[]; channels: Channel[]; listRef?: Ref<HTMLUListElement> }) {
  return (
    <ul className="crew-results" aria-label="Results" tabIndex={-1} ref={listRef}>
      {outcomes.map((o) => {
        const { state, tone } = describe(o)
        const name = channels.find((c) => c.id === o.channelId)?.name ?? serviceName(o.service)
        return <Row key={o.channelId} name={name} state={state} tone={tone} />
      })}
    </ul>
  )
}
