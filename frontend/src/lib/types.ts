export interface Substation {
  id: string
  lat: number
  lon: number
}

export interface Feeder {
  id: string
  substation_id: string
  name: string
  lat: number
  lon: number
}

export interface Transformer {
  id: string
  feeder_id: string
  lat: number
  lon: number
  capacity_kva: number
  households_served: number
}

export interface GTPole {
  id: string
  dt_id: string
  parent_pole_id: string | null
  seq_on_line: number
  is_branch_point: boolean
  lat: number
  lon: number
}

export interface GTTopologyNode {
  pole_id: string
  parent_id: string | null
  dt_id: string
  seq_on_line: number
  children: string[]
  is_branch_point: boolean
  lat: number
  lon: number
}

export interface RegistryPole {
  id: string
  dt_id: string
  feeder_id: string
  lat: number
  lon: number
  seq_on_line?: number | null
  parent_pole_id?: string | null
  pole_type: string
  ward: string
  pincode?: string | null
  device_id?: string | null
}

export interface DTTopologyStatus {
  dt_id: string
  has_topology: boolean
}

export interface NetworkData {
  substations: Substation[]
  feeders: Feeder[]
  transformers: Transformer[]
  gt_topology: GTTopologyNode[]
  registry_poles: RegistryPole[]
  dt_topology_status: DTTopologyStatus[]
}

export type TicketStatus = "active" | "verified" | "resolved"
export type TicketSeverity = "critical" | "major" | "minor"
export type TicketScope = "span" | "dt" | "feeder"

export interface Location {
  lat: number
  lon: number
}

export interface TicketEvent {
  event: string
  energized: boolean
  reported: boolean
  ts: string
  seq: number
  battery_mv: number
  rssi: number
  fw: string
  received_at: string
  pole_id: string
  device_id: string
}

export interface Ticket {
  id: string
  status: TicketStatus
  severity: TicketSeverity
  scope: TicketScope
  target_id: string
  dt_id: string
  feeder_id: string
  affected_count: number
  affected_poles: string[]
  confidence: number
  evidence: TicketEvent[]
  detected_at: string
  verified_at: string | null
  resolved_at: string | null
  pincode: string | null
  location: Location | null
  is_refined: boolean
}
