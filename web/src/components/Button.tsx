import type { ButtonHTMLAttributes } from 'react'
import './components.css'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary'
  busy?: boolean
}

export default function Button({ variant = 'primary', busy, className, type = 'button', ...rest }: Props) {
  const cls = ['btn', `btn-${variant}`, className].filter(Boolean).join(' ')
  // While a request is in flight the label stays and the button is aria-disabled, not disabled, so focus is kept.
  return <button type={type} className={cls} aria-busy={busy || undefined} aria-disabled={busy || undefined} {...rest} />
}
