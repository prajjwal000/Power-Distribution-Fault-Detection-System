import { useState, useEffect, useCallback } from "react"
import { injectFault, repairFault, repairAllFaults, listFaults, type Fault } from "@/api/simulator"

export interface FaultState {
  faults: Fault[]
  loading: boolean
  error: Error | null
  injectFault: (target: { type: "span" | "dt" | "feeder"; parent_id?: string; child_id?: string; target_id?: string }) => Promise<Fault | null>
  repairFault: (id: string) => Promise<void>
  repairAll: () => Promise<void>
  refresh: () => Promise<void>
}

export function useFaults(): FaultState {
  const [faults, setFaults] = useState<Fault[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await listFaults()
      setFaults(data)
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
    } finally {
      setLoading(false)
    }
  }, [])

// eslint-disable-next-line react-hooks/set-state-in-effect, react-hooks/exhaustive-deps
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    refresh()
  }, [])

  const doInjectFault = useCallback(async (target: { type: "span" | "dt" | "feeder"; parent_id?: string; child_id?: string; target_id?: string }) => {
    setError(null)
    try {
      const fault = await injectFault(target)
      await refresh()
      return fault
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
      return null
    }
  }, [refresh])

  const doRepairFault = useCallback(async (id: string) => {
    setError(null)
    try {
      await repairFault(id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
    }
  }, [refresh])

  const doRepairAll = useCallback(async () => {
    setError(null)
    try {
      await repairAllFaults()
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
    }
  }, [refresh])

  return { faults, loading, error, injectFault: doInjectFault, repairFault: doRepairFault, repairAll: doRepairAll, refresh }
}