import { useState, useCallback } from "react"
import { injectNoise, killDevice, resumeDevice, fetchSimStats, type NoiseRequest, type TelemetryStats } from "@/api/simulator"

export interface NoiseState {
  loading: boolean
  error: Error | null
  injectNoise: (req: NoiseRequest) => Promise<void>
  killDevice: (deviceId: string, autoResumeSecs?: number) => Promise<void>
  resumeDevice: (deviceId: string) => Promise<void>
  fetchStats: () => Promise<TelemetryStats | null>
}

export function useNoise(): NoiseState {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const doInjectNoise = useCallback(async (req: NoiseRequest) => {
    setLoading(true)
    setError(null)
    try {
      await injectNoise(req)
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
      throw e
    } finally {
      setLoading(false)
    }
  }, [])

  const doKillDevice = useCallback(async (deviceId: string, autoResumeSecs?: number) => {
    setLoading(true)
    setError(null)
    try {
      await killDevice(deviceId, autoResumeSecs)
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
      throw e
    } finally {
      setLoading(false)
    }
  }, [])

  const doResumeDevice = useCallback(async (deviceId: string) => {
    setLoading(true)
    setError(null)
    try {
      await resumeDevice(deviceId)
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
      throw e
    } finally {
      setLoading(false)
    }
  }, [])

  const doFetchStats = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      return await fetchSimStats()
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  return { loading, error, injectNoise: doInjectNoise, killDevice: doKillDevice, resumeDevice: doResumeDevice, fetchStats: doFetchStats }
}