import { useEffect, useState, useCallback } from "react"
import { fetchClock, setClock, type ClockState } from "@/api/simulator"

export interface SimClockState extends ClockState {
  setMultiplier: (m: number) => void
  pause: () => void
  resume: () => void
  loading: boolean
  error: Error | null
}

export function useSimClock(): SimClockState {
  const [state, setState] = useState<ClockState>({
    sim_time: "00:00:00",
    multiplier: 30,
    wall_time: "",
    paused: false,
  })
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    let cancelled = false

    function load() {
      fetchClock()
        .then((clock) => {
          if (cancelled) return
          setState(clock)
          setError(null)
        })
        .catch((err) => {
          if (cancelled) return
          setError(err instanceof Error ? err : new Error(String(err)))
        })
    }

    load()
    const interval = setInterval(load, 1000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [])

  const setMultiplier = useCallback((m: number) => {
    setClock({ multiplier: m })
      .then((clock) => {
        setState(clock)
      })
      .catch((err) => {
        setError(err instanceof Error ? err : new Error(String(err)))
      })
  }, [])

  const pause = useCallback(() => {
    setClock({ paused: true })
      .then((clock) => {
        setState(clock)
      })
      .catch((err) => {
        setError(err instanceof Error ? err : new Error(String(err)))
      })
  }, [])

  const resume = useCallback(() => {
    setClock({ paused: false })
      .then((clock) => {
        setState(clock)
      })
      .catch((err) => {
        setError(err instanceof Error ? err : new Error(String(err)))
      })
  }, [])

  return { ...state, setMultiplier, pause, resume, loading: false, error }
}
