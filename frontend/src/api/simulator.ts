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
