import { useEffect, useRef } from 'react'

// A <details> menu that also closes on an outside click, on Escape (focus goes back to the
// button) and when an item is chosen, so it never lingers over the page.
export default function AccountMenu() {
  const box = useRef<HTMLDetailsElement>(null)

  useEffect(() => {
    const el = box.current
    if (!el) return
    const close = () => {
      el.open = false
    }
    const outside = (e: PointerEvent) => {
      if (el.open && e.target instanceof Node && !el.contains(e.target)) close()
    }
    const key = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || !el.open) return
      close()
      el.querySelector('summary')?.focus()
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', key)
    }
  }, [])

  return (
    <details className="cap-account" ref={box}>
      <summary>Account</summary>
      <div className="cap-account-menu" onClick={() => box.current && (box.current.open = false)}>
        <a href="/settings">Settings</a>
        <a href="/auth/logout">Sign out</a>
      </div>
    </details>
  )
}
