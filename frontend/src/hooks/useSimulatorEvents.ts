import { useEffect, useState } from "react"

export interface TelemetryEvent {
  device_id: string
  pole_id: string
  event: "heartbeat" | "power_lost" | "power_restored" | "boot"
  energized: boolean
  ts: string
  seq: number
  battery_mv: number
  rssi: number
  fw: string
}

export interface PoleState {
  energized: boolean
  lastEvent: TelemetryEvent["event"]
  lastEventTime: number
  lastEventSeq: number
}

export interface SimEventsState {
  poleStates: Map<string, PoleState>
  connected: boolean
  lastEvent: TelemetryEvent | null
}

export function useSimulatorEvents(): SimEventsState {
  const [poleStates, setPoleStates] = useState(new Map<string, PoleState>())
  const [connected, setConnected] = useState(false)
  const [lastEvent, setLastEvent] = useState<TelemetryEvent | null>(null)

  useEffect(() => {
    const es = new EventSource("/sim/events/stream")

    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    es.onmessage = (e) => {
      try {
        const evt = JSON.parse(e.data) as TelemetryEvent

        setPoleStates((prev) => {
          const next = new Map(prev)
          next.set(evt.pole_id, {
            energized: evt.energized,
            lastEvent: evt.event,
            lastEventTime: Date.now(),
            lastEventSeq: evt.seq,
          })
          return next
        })

        setLastEvent(evt)
      } catch {
        // ignore malformed events
      }
    }

    return () => {
      es.close()
    }
  }, [])

  return { poleStates, connected, lastEvent }
}

