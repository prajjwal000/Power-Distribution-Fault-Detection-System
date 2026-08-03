import type { NetworkData } from "@/lib/types"

export const mockNetwork: NetworkData = {
  substations: [
    { id: "S-001", lat: 12.9716, lon: 77.5946 },
  ],

  feeders: [
    { id: "F-001-01", substation_id: "S-001", name: "Feeder 1-1", lat: 12.973, lon: 77.596 },
    { id: "F-001-02", substation_id: "S-001", name: "Feeder 1-2", lat: 12.970, lon: 77.593 },
  ],

  transformers: [
    { id: "D-0001", feeder_id: "F-001-01", lat: 12.974, lon: 77.597, capacity_kva: 200, households_served: 180 },
    { id: "D-0002", feeder_id: "F-001-02", lat: 12.971, lon: 77.594, capacity_kva: 150, households_served: 120 },
    { id: "D-0003", feeder_id: "F-001-02", lat: 12.969, lon: 77.591, capacity_kva: 250, households_served: 200 },
  ],

  dt_topology_status: [
    { dt_id: "D-0001", has_topology: true },
    { dt_id: "D-0002", has_topology: false },
    { dt_id: "D-0003", has_topology: true },
  ],

  // Ground truth topology — complete parent-child tree per DT
  // D-0001: main line with two branch points
  //   P-001 -> P-002 -> P-005 (branch) -> P-008, P-009
  //                       P-006 -> P-010
  //          P-003 -> P-004
  //          P-007
  //
  // D-0002: Y-fork — one branch point
  //   P-011 -> P-012 (branch) -> P-015
  //                       P-016 -> P-017
  //          P-013 -> P-014
  //
  // D-0003: short straight line (no branches)
  //   P-018 -> P-019 -> P-020 -> P-021
  gt_topology: [
    // ── D-0001 ──
    { pole_id: "P-001", parent_id: null,              dt_id: "D-0001", seq_on_line: 1,  children: ["P-002", "P-003", "P-007"], is_branch_point: true,  lat: 12.9745, lon: 77.5972 },
    { pole_id: "P-002", parent_id: "P-001",           dt_id: "D-0001", seq_on_line: 2,  children: ["P-005", "P-006"],             is_branch_point: true,  lat: 12.9749, lon: 77.5976 },
    { pole_id: "P-003", parent_id: "P-001",           dt_id: "D-0001", seq_on_line: 3,  children: ["P-004"],                      is_branch_point: false, lat: 12.9743, lon: 77.5968 },
    { pole_id: "P-004", parent_id: "P-003",           dt_id: "D-0001", seq_on_line: 4,  children: [],                             is_branch_point: false, lat: 12.9741, lon: 77.5964 },
    { pole_id: "P-005", parent_id: "P-002",           dt_id: "D-0001", seq_on_line: 5,  children: ["P-008", "P-009"],             is_branch_point: true,  lat: 12.9753, lon: 77.5980 },
    { pole_id: "P-006", parent_id: "P-002",           dt_id: "D-0001", seq_on_line: 6,  children: ["P-010"],                      is_branch_point: false, lat: 12.9751, lon: 77.5973 },
    { pole_id: "P-007", parent_id: "P-001",           dt_id: "D-0001", seq_on_line: 7,  children: [],                             is_branch_point: false, lat: 12.9742, lon: 77.5975 },
    { pole_id: "P-008", parent_id: "P-005",           dt_id: "D-0001", seq_on_line: 8,  children: [],                             is_branch_point: false, lat: 12.9757, lon: 77.5984 },
    { pole_id: "P-009", parent_id: "P-005",           dt_id: "D-0001", seq_on_line: 9,  children: [],                             is_branch_point: false, lat: 12.9755, lon: 77.5978 },
    { pole_id: "P-010", parent_id: "P-006",           dt_id: "D-0001", seq_on_line: 10, children: [],                             is_branch_point: false, lat: 12.9754, lon: 77.5977 },

    // ── D-0002 ──
    { pole_id: "P-011", parent_id: null,              dt_id: "D-0002", seq_on_line: 1,  children: ["P-012", "P-013"],             is_branch_point: true,  lat: 12.9715, lon: 77.5942 },
    { pole_id: "P-012", parent_id: "P-011",           dt_id: "D-0002", seq_on_line: 2,  children: ["P-015", "P-016"],             is_branch_point: true,  lat: 12.9719, lon: 77.5946 },
    { pole_id: "P-013", parent_id: "P-011",           dt_id: "D-0002", seq_on_line: 3,  children: ["P-014"],                      is_branch_point: false, lat: 12.9713, lon: 77.5938 },
    { pole_id: "P-014", parent_id: "P-013",           dt_id: "D-0002", seq_on_line: 4,  children: [],                             is_branch_point: false, lat: 12.9711, lon: 77.5934 },
    { pole_id: "P-015", parent_id: "P-012",           dt_id: "D-0002", seq_on_line: 5,  children: [],                             is_branch_point: false, lat: 12.9723, lon: 77.5950 },
    { pole_id: "P-016", parent_id: "P-012",           dt_id: "D-0002", seq_on_line: 6,  children: ["P-017"],                      is_branch_point: false, lat: 12.9721, lon: 77.5943 },
    { pole_id: "P-017", parent_id: "P-016",           dt_id: "D-0002", seq_on_line: 7,  children: [],                             is_branch_point: false, lat: 12.9724, lon: 77.5947 },

    // ── D-0003 ──
    { pole_id: "P-018", parent_id: null,              dt_id: "D-0003", seq_on_line: 1,  children: ["P-019"],                      is_branch_point: false, lat: 12.9695, lon: 77.5912 },
    { pole_id: "P-019", parent_id: "P-018",           dt_id: "D-0003", seq_on_line: 2,  children: ["P-020"],                      is_branch_point: false, lat: 12.9693, lon: 77.5908 },
    { pole_id: "P-020", parent_id: "P-019",           dt_id: "D-0003", seq_on_line: 3,  children: ["P-021"],                      is_branch_point: false, lat: 12.9691, lon: 77.5904 },
    { pole_id: "P-021", parent_id: "P-020",           dt_id: "D-0003", seq_on_line: 4,  children: [],                             is_branch_point: false, lat: 12.9689, lon: 77.5900 },
  ],

  // Registry poles — mirrors GT but with device_id gaps and missing topology for D-0002
  registry_poles: [
    // ── D-0001 (has_topology = true → parent/seq present) ──
    { id: "P-001", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9745, lon: 77.5972, seq_on_line: 1,  parent_pole_id: null,          pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-001" },
    { id: "P-002", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9749, lon: 77.5976, seq_on_line: 2,  parent_pole_id: "P-001",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-002" },
    { id: "P-003", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9743, lon: 77.5968, seq_on_line: 3,  parent_pole_id: "P-001",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560002", device_id: "KSPDB-D-0001-P-003" },
    { id: "P-004", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9741, lon: 77.5964, seq_on_line: 4,  parent_pole_id: "P-003",       pole_type: "LT-9m-PCC", ward: "W-013", pincode: "560002", device_id: "KSPDB-D-0001-P-004" },
    { id: "P-005", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9753, lon: 77.5980, seq_on_line: 5,  parent_pole_id: "P-002",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-005" },
    { id: "P-006", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9751, lon: 77.5973, seq_on_line: 6,  parent_pole_id: "P-002",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-006" },
    { id: "P-007", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9742, lon: 77.5975, seq_on_line: 7,  parent_pole_id: "P-001",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-007" },
    { id: "P-008", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9757, lon: 77.5984, seq_on_line: 8,  parent_pole_id: "P-005",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-008" },
    { id: "P-009", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9755, lon: 77.5978, seq_on_line: 9,  parent_pole_id: "P-005",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-009" },
    { id: "P-010", dt_id: "D-0001", feeder_id: "F-001-01", lat: 12.9754, lon: 77.5977, seq_on_line: 10, parent_pole_id: "P-006",       pole_type: "LT-9m-PCC", ward: "W-012", pincode: "560001", device_id: "KSPDB-D-0001-P-010" },

    // ── D-0002 (has_topology = false → parent/seq MISSING) ──
    { id: "P-011", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9715, lon: 77.5942, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-011" },
    { id: "P-012", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9719, lon: 77.5946, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-012" },
    { id: "P-013", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9713, lon: 77.5938, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-013" },
    // P-014: no device (simulates ~9% no-device poles)
    { id: "P-014", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9711, lon: 77.5934, pole_type: "LT-9m-PCC", ward: "W-045" },
    { id: "P-015", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9723, lon: 77.5950, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-015" },
    { id: "P-016", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9721, lon: 77.5943, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-016" },
    { id: "P-017", dt_id: "D-0002", feeder_id: "F-001-02", lat: 12.9724, lon: 77.5947, pole_type: "LT-9m-PCC", ward: "W-045", device_id: "KSPDB-D-0002-P-017" },

    // ── D-0003 (has_topology = true → parent/seq present) ──
    { id: "P-018", dt_id: "D-0003", feeder_id: "F-001-02", lat: 12.9695, lon: 77.5912, seq_on_line: 1,  parent_pole_id: null,      pole_type: "LT-9m-PCC", ward: "W-078", pincode: "560004", device_id: "KSPDB-D-0003-P-018" },
    { id: "P-019", dt_id: "D-0003", feeder_id: "F-001-02", lat: 12.9693, lon: 77.5908, seq_on_line: 2,  parent_pole_id: "P-018",   pole_type: "LT-9m-PCC", ward: "W-078", pincode: "560004", device_id: "KSPDB-D-0003-P-019" },
    // P-020: no device
    { id: "P-020", dt_id: "D-0003", feeder_id: "F-001-02", lat: 12.9691, lon: 77.5904, seq_on_line: 3,  parent_pole_id: "P-019",   pole_type: "LT-9m-PCC", ward: "W-078", pincode: "560004" },
    { id: "P-021", dt_id: "D-0003", feeder_id: "F-001-02", lat: 12.9689, lon: 77.5900, seq_on_line: 4,  parent_pole_id: "P-020",   pole_type: "LT-9m-PCC", ward: "W-078", pincode: "560005", device_id: "KSPDB-D-0003-P-021" },
  ],
}
