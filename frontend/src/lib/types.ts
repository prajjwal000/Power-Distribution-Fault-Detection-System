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
