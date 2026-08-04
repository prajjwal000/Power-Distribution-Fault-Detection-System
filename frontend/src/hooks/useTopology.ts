import { useEffect, useState } from "react"
import { fetchTopology, SimulatorError } from "@/api/simulator"
import type { NetworkData } from "@/lib/types"

export interface TopologyState {
  data: NetworkData | null
  loading: boolean
  error: Error | null
}

export function useTopology(): TopologyState {
  const [data, setData] = useState<NetworkData | null>(null)
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    let cancelled = false

    fetchTopology()
      .then((network) => {
        if (cancelled) return
        setData(network)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err instanceof SimulatorError ? err : new Error(String(err)))
      })

    return () => {
      cancelled = true
    }
  }, [])

  return { data, loading: data === null && error === null, error }
}
