import { useEffect, useState } from 'react'
import { ApiError } from './api/errors'

export function useLoad<T>(load: () => Promise<T>, deps: readonly unknown[]) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    load().then(
      (value) => {
        if (cancelled) return
        setData(value)
        setError(null)
        setLoading(false)
      },
      (reason: unknown) => {
        if (cancelled) return
        setError(reason instanceof ApiError ? reason.code : 'unknown')
        setLoading(false)
      },
    )
    return () => {
      cancelled = true
    }
    // The caller passes the values load closes over.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick])

  return { data, error, loading, reload: () => setTick((value) => value + 1) }
}
