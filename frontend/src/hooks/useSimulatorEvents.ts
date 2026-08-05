import { useEffect, useState, useRef, useCallback } from "react"

export interface TelemetryEvent {
  device_id: string
  pole_id: string
  event: "heartbeat" | "power_lost" | "power_restored" | "boot"
  energized: boolean
  reported: boolean
  ts: string
  seq: number
  battery_mv: number
  rssi: number
  fw: string
}

export interface PoleState {
  energized: boolean
  reported: boolean
  lastEvent: TelemetryEvent["event"]
  lastEventTime: number
  lastEventSeq: number
}

export interface SimEventsState {
  poleStates: Map<string, PoleState>
  connected: boolean
  lastEvents: TelemetryEvent[]
}

const BATCH_INTERVAL_MS = 100 // flush at ~10Hz

export function useSimulatorEvents(): SimEventsState {
  const [poleStates, setPoleStates] = useState(new Map<string, PoleState>())
  const [connected, setConnected] = useState(false)
  const [lastEvents, setLastEvents] = useState<TelemetryEvent[]>([])

  const bufferRef = useRef<TelemetryEvent[]>([])
  const flushTimerRef = useRef<number | null>(null)

  const flush = useCallback(() => {
    if (bufferRef.current.length === 0) return

    const batch = bufferRef.current
    bufferRef.current = []

    // Update poleStates in one batched setState
    setPoleStates((prev) => {
      const next = new Map(prev)
      for (const evt of batch) {
        next.set(evt.pole_id, {
          energized: evt.energized,
          reported: evt.reported,
          lastEvent: evt.event,
          lastEventTime: Date.now(),
          lastEventSeq: evt.seq,
        })
      }
      return next
    })

    // Pass entire batch to drive pulse animations for every event
    setLastEvents(batch)
  }, [])

  useEffect(() => {
    const es = new EventSource("/sim/events/stream")

    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    es.onmessage = (e) => {
      try {
        const evt = JSON.parse(e.data) as TelemetryEvent
        bufferRef.current.push(evt)

        // Start flush timer if not already running
        if (flushTimerRef.current === null) {
          flushTimerRef.current = window.setInterval(flush, BATCH_INTERVAL_MS)
        }
      } catch {
        // ignore malformed events
      }
    }

    return () => {
      es.close()
      if (flushTimerRef.current !== null) {
        clearInterval(flushTimerRef.current)
        flushTimerRef.current = null
      }
      // Flush any remaining
      flush()
    }
  }, [flush])

  return { poleStates, connected, lastEvents }
}