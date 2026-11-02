import { useCallback, useEffect, useRef, useState } from 'react'
import { getLaunch, patchLaunch, type ApiError, type Detail, type Patch } from '../api'
import { asError } from './format'

const REFETCH = 15000

// Detail loads on mount, after each PATCH and on demand. It polls only once launched:
// on a draft every GET calls Buffer, so a draft is never polled.
export function useLaunch(id: string) {
  const [detail, setDetail] = useState<Detail>()
  const [error, setError] = useState<ApiError>()
  const live = useRef(true)

  useEffect(() => {
    live.current = true
    return () => {
      live.current = false
    }
  }, [])

  const reload = useCallback(async () => {
    try {
      const d = await getLaunch(id)
      if (live.current) {
        setDetail(d)
        setError(undefined)
      }
    } catch (e) {
      if (live.current) setError(asError(e))
    }
  }, [id])

  useEffect(() => {
    reload()
  }, [reload])

  const launched = detail?.status === 'launched'
  useEffect(() => {
    if (!launched) return
    const t = window.setInterval(reload, REFETCH)
    return () => window.clearInterval(t)
  }, [launched, reload])

  const patch = useCallback(
    async (p: Patch) => {
      const d = await patchLaunch(id, p)
      if (live.current) setDetail(d)
    },
    [id],
  )

  return { detail, error, reload, patch }
}
