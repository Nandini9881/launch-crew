import type { ReactNode } from 'react'
import './components.css'

type Tone = 'info' | 'success' | 'warning' | 'error'
type Props = { tone?: Tone; role?: 'status' | 'alert'; lead?: string; children: ReactNode }

export default function Alert({ tone = 'info', role, lead, children }: Props) {
  return (
    <div className={`alert alert-${tone}`} role={role ?? (tone === 'error' ? 'alert' : 'status')}>
      {lead && <strong className="alert-lead">{lead} </strong>}
      {children}
    </div>
  )
}
