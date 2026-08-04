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
