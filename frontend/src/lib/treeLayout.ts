import { hierarchy, tree, type HierarchyPointNode } from "d3-hierarchy"
import type { NetworkData, GTTopologyNode } from "./types"

// ── Tree node shape fed into d3.hierarchy ──

export interface TreeNode {
  id: string
  type: "root" | "substation" | "feeder" | "dt" | "pole"
  name: string
  children: TreeNode[]
  // metadata for rendering
  dtId?: string
  feederId?: string
  deviceId?: string | null
  isBranchPoint?: boolean
  hasDevice?: boolean
  hasTopology?: boolean
  downstreamCount?: number
}

// ── Output types ──

export interface PositionedNode {
  id: string
  type: TreeNode["type"]
  name: string
  x: number
  y: number
  depth: number
  dtId?: string
  feederId?: string
  deviceId?: string | null
  isBranchPoint?: boolean
  hasDevice?: boolean
  hasTopology?: boolean
  downstreamCount?: number
  collapsed: boolean
  hasChildren: boolean
}

export interface PositionedLink {
  sourceId: string
  targetId: string
  x1: number
  y1: number
  x2: number
  y2: number
  known: boolean // true if edge exists in registry
}

export interface LayoutResult {
  nodes: PositionedNode[]
  links: PositionedLink[]
}

// ── Internal d3 node type with stash support ──

type D3Node = HierarchyPointNode<TreeNode> & {
  _children?: D3Node[]
  x0?: number
  y0?: number
}

// ── Build nested tree from flat network data ──

function buildTree(data: NetworkData): TreeNode {
  // Index GT poles by dt_id for fast child lookup
  const polesByDt = new Map<string, GTTopologyNode[]>()
  for (const pole of data.gt_topology) {
    const list = polesByDt.get(pole.dt_id) ?? []
    list.push(pole)
    polesByDt.set(pole.dt_id, list)
  }

  // Index registry poles for device info
  const registryById = new Map(data.registry_poles.map((p) => [p.id, p]))

  // Index DT topology status
  const topoStatus = new Map(data.dt_topology_status.map((s) => [s.dt_id, s.has_topology]))

  // Build pole subtrees (recursive)
  function buildPoleTree(
    poleId: string,
    dtId: string,
    poles: GTTopologyNode[],
    visited: Set<string>,
  ): TreeNode | null {
    if (visited.has(poleId)) return null
    visited.add(poleId)

    const gt = poles.find((p) => p.pole_id === poleId)
    if (!gt) return null

    const reg = registryById.get(poleId)
    const poleChildren: TreeNode[] = []
    for (const childId of gt.children) {
      const child = buildPoleTree(childId, dtId, poles, visited)
      if (child) poleChildren.push(child)
    }

    return {
      id: poleId,
      type: "pole",
      name: poleId,
      children: poleChildren,
      dtId,
      deviceId: reg?.device_id ?? null,
      isBranchPoint: gt.is_branch_point,
      hasDevice: reg?.device_id != null,
      downstreamCount: poleChildren.length > 0 ? undefined : undefined,
    }
  }

  // Assemble full tree
  const rootChildren: TreeNode[] = []

  for (const sub of data.substations) {
    const feederChildren: TreeNode[] = []

    for (const feeder of data.feeders.filter((f) => f.substation_id === sub.id)) {
      const dtChildren: TreeNode[] = []

      for (const dt of data.transformers.filter((t) => t.feeder_id === feeder.id)) {
        const poles = polesByDt.get(dt.id) ?? []
        const visited = new Set<string>()

        // Find root poles (no parent)
        const rootPoles = poles.filter((p) => p.parent_id === null)

        const poleChildren: TreeNode[] = []
        for (const rp of rootPoles) {
          const tree = buildPoleTree(rp.pole_id, dt.id, poles, visited)
          if (tree) poleChildren.push(tree)
        }

        dtChildren.push({
          id: dt.id,
          type: "dt",
          name: dt.id,
          children: poleChildren,
          feederId: feeder.id,
          hasTopology: topoStatus.get(dt.id) ?? false,
          downstreamCount: poles.length,
        })
      }

      feederChildren.push({
        id: feeder.id,
        type: "feeder",
        name: feeder.name,
        children: dtChildren,
        downstreamCount: data.transformers.filter((t) => t.feeder_id === feeder.id).length,
      })
    }

    rootChildren.push({
      id: sub.id,
      type: "substation",
      name: sub.id,
      children: feederChildren,
      downstreamCount: data.feeders.filter((f) => f.substation_id === sub.id).length,
    })
  }

  return {
    id: "__root__",
    type: "root",
    name: "KSPDB",
    children: rootChildren,
  }
}

// ── Determine edge "known" status from registry ──

function isEdgeKnown(
  childId: string,
  registryById: Map<string, { parent_pole_id?: string | null }>,
): boolean {
  const child = registryById.get(childId)
  if (!child) return false
  // For poles: edge is known if child has parent_pole_id in registry
  // For non-pole nodes (DT→feeder, feeder→substation): always known
  return child.parent_pole_id != null
}

// ── Compute layout ──

// Horizontal spacing between depth levels
const DX = 220
// Vertical spacing between sibling nodes
const DY = 32

export function computeLayout(data: NetworkData, collapsedIds: Set<string>): LayoutResult {
  const root = buildTree(data)
  const registryById = new Map(data.registry_poles.map((p) => [p.id, p]))

  // Build d3 hierarchy
  const hRoot = hierarchy(root, (d) => (d.children.length > 0 ? d.children : undefined))

  // Apply collapse: stash children into _children for collapsed nodes
  hRoot.each((node) => {
    const d3node = node as D3Node
    if (collapsedIds.has(d3node.data.id) && d3node.children) {
      d3node._children = d3node.children as D3Node[]
      d3node.children = undefined
    }
  })

  // Run tree layout
  const layout = tree<TreeNode>().nodeSize([DY, DX])
  layout(hRoot)

  // Collect positioned nodes
  const nodes: PositionedNode[] = []
  hRoot.each((node) => {
    const d3node = node as D3Node
    const d = d3node.data
    nodes.push({
      id: d.id,
      type: d.type,
      name: d.name,
      x: d3node.y,           // depth → canvas x (left-to-right)
      y: d3node.x + 300,     // sibling order → canvas y (+ offset so root starts at left)
      depth: d3node.depth,
      dtId: d.dtId,
      feederId: d.feederId,
      deviceId: d.deviceId,
      isBranchPoint: d.isBranchPoint,
      hasDevice: d.hasDevice,
      hasTopology: d.hasTopology,
      downstreamCount: d.downstreamCount,
      collapsed: collapsedIds.has(d.id),
      hasChildren: d.children.length > 0 || (d3node._children?.length ?? 0) > 0,
    })
  })

  // Collect positioned links
  const links: PositionedLink[] = []
  hRoot.links().forEach((link) => {
    const src = link.source as D3Node
    const tgt = link.target as D3Node
    links.push({
      sourceId: src.data.id,
      targetId: tgt.data.id,
      x1: src.y,
      y1: src.x + 300,
      x2: tgt.y,
      y2: tgt.x + 300,
      known: isEdgeKnown(tgt.data.id, registryById),
    })
  })

  return { nodes, links }
}
