import { useEffect, useState } from 'react'

const MINUTE = 60_000

// The current time, refreshed on an interval, so countdowns rendered from it keep
// moving ("2h 14m" ticks down and turns "Overdue") without the page being reloaded.
export function useNow(every = MINUTE) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), every)
    return () => window.clearInterval(t)
  }, [every])
  return now
}
