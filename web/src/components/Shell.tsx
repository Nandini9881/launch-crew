import type { ReactNode } from 'react'
import '../styles/layout.css'

export default function Shell({ width, id, children }: { width: 'person' | 'captain'; id?: string; children: ReactNode }) {
  return (
    <main className="shell" data-width={width} id={id}>
      {children}
    </main>
  )
}
