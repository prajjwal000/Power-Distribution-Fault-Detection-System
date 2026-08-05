import type { NetworkData } from "@/lib/types"

export class SimulatorError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = "SimulatorError"
    this.status = status
  }
}

export async function fetchTopology(): Promise<NetworkData> {
  const res = await fetch("/sim/topology/tree")
  if (!res.ok) {
    throw new SimulatorError(
      `Failed to fetch topology: ${res.status} ${res.statusText}`,
      res.status,
    )
  }
  return (await res.json()) as NetworkData
}

export interface ClockState {
  sim_time: string
  multiplier: number
  wall_time: string
  paused: boolean
}

export async function fetchClock(): Promise<ClockState> {
  const res = await fetch("/clock")
  if (!res.ok) {
    throw new SimulatorError(`Failed to fetch clock: ${res.status} ${res.statusText}`, res.status)
  }
  return (await res.json()) as ClockState
}

export interface SetClockParams {
  multiplier?: number
  sim_time?: number
  paused?: boolean
}

export async function setClock(params: SetClockParams): Promise<ClockState> {
  const res = await fetch("/clock", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  })
  if (!res.ok) {
    throw new SimulatorError(`Failed to set clock: ${res.status} ${res.statusText}`, res.status)
  }
  return (await res.json()) as ClockState
}

// ── Fault injection ── //

export interface FaultTarget {
  type: "span" | "dt" | "feeder"
  parent_id?: string
  child_id?: string
  target_id?: string
  auto_repair_secs?: number
}

export interface Fault {
  id: string
  type: "span" | "dt" | "feeder"
  target: string
  affected_poles: string[]
  affected_count: number
  start_sim: number
  auto_repair_sim_secs?: number
  repair_at_sim?: number
}

export async function injectFault(target: FaultTarget): Promise<Fault | null> {
  const res = await fetch("/sim/faults", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(target),
  })
  if (res.status === 409) {
    // Duplicate fault — not an error, just return null
    return null
  }
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to inject fault: ${res.status} ${msg}`, res.status)
  }
  return (await res.json()) as Fault
}

export async function repairFault(id: string): Promise<void> {
  const res = await fetch(`/sim/faults/${id}`, { method: "POST" })
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to repair fault: ${res.status} ${msg}`, res.status)
  }
}

export async function repairAllFaults(): Promise<void> {
  const res = await fetch("/sim/faults/repair", { method: "POST" })
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to repair all faults: ${res.status} ${msg}`, res.status)
  }
}

export async function listFaults(): Promise<Fault[]> {
  const res = await fetch("/sim/faults")
  if (!res.ok) {
    throw new SimulatorError(`Failed to list faults: ${res.status} ${res.statusText}`, res.status)
  }
  return (await res.json()) as Fault[]
}

// ── Noise injection ── //

export interface NoiseRequest {
  kind: "device_death" | "duplicate" | "stale_replay"
  device_id?: string
  count?: number
  auto_resume_secs?: number
}

export async function injectNoise(req: NoiseRequest): Promise<void> {
  const res = await fetch("/sim/noise", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to inject noise: ${res.status} ${msg}`, res.status)
  }
}

// ── Device kill/resume ── //

export async function killDevice(deviceId: string, autoResumeSecs?: number): Promise<void> {
  const res = await fetch(`/sim/devices/${deviceId}/kill`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ auto_resume_secs: autoResumeSecs }),
  })
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to kill device: ${res.status} ${msg}`, res.status)
  }
}

export async function resumeDevice(deviceId: string): Promise<void> {
  const res = await fetch(`/sim/devices/${deviceId}/resume`, { method: "POST" })
  if (!res.ok) {
    const msg = await res.text()
    throw new SimulatorError(`Failed to resume device: ${res.status} ${msg}`, res.status)
  }
}

// ── Stats ── //

export interface TelemetryStats {
  events_attempted: number
  events_delivered: number
  power_lost_attempted: number
  power_lost_delivered: number
  device_deaths: number
  device_resumes: number
}

export async function fetchSimStats(): Promise<TelemetryStats> {
  const res = await fetch("/sim/stats")
  if (!res.ok) {
    throw new SimulatorError(`Failed to fetch stats: ${res.status} ${res.statusText}`, res.status)
  }
  return (await res.json()) as TelemetryStats
}

// ── Scheduled outages ── //

export interface ScheduledOutage {
  id: string
  scope: "feeder" | "dt"
  target_id: string
  start: string
  end: string
  reason: string
}

export async function fetchScheduledOutages(from: string, to: string): Promise<ScheduledOutage[]> {
  const res = await fetch(`/scheduled-outages?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
  if (!res.ok) {
    throw new SimulatorError(`Failed to fetch scheduled outages: ${res.status} ${res.statusText}`, res.status)
  }
  return (await res.json()) as ScheduledOutage[]
}
