import { useRef, useEffect, useLayoutEffect, useCallback, useState } from "react"
import type { PositionedNode, PositionedLink } from "@/lib/treeLayout"
import type { PoleState, TelemetryEvent } from "@/hooks/useSimulatorEvents"
import type { Fault } from "@/api/simulator"

export interface CanvasSelection {
  type: "node" | "edge"
  id: string
}

interface ActivePulse {
  poleId: string
  event: TelemetryEvent["event"]
  startTime: number
  duration: number
}

interface NetworkCanvasProps {
  nodes: PositionedNode[]
  links: PositionedLink[]
  selection: CanvasSelection[]
  transform: { x: number; y: number; k: number } | null
  onSelect: (sel: CanvasSelection[], multiSelect: boolean) => void
  onToggleCollapse: (nodeId: string) => void
  onTransformChange: (t: { x: number; y: number; k: number }) => void
  poleStates?: Map<string, PoleState>
  lastEvents?: TelemetryEvent[]
  activeFaults?: Fault[]
}

// ── Layout constants (must match treeLayout.ts) ──

const NODE_RADIUS = 6
const CONTAINER_W = 140
const CONTAINER_H = 28
const INDICATOR_SIZE = 8

// ── Colors ──

const COLORS = {
  bg: "#0a0a0a",
  edge: "#404040",
  edgeKnown: "#555",
  edgeDashed: "#666",
  edgeSelected: "#f97316",
  nodeStroke: "#d4d4d4",
  nodeFill: "#1a1a1a",
  nodeSelected: "#f97316",
  substationFill: "#1e3a5f",
  feederFill: "#2d5016",
  dtFill: "#5c1a1a",
  poleFill: "#d4d4d4",
  poleNoDevice: "transparent",
  poleDark: "#991b1b",
  poleSilentFail: "#92400e",
  text: "#e5e5e5",
  textMuted: "#737373",
  indicatorFill: "#888",
}

const PULSE_COLORS: Record<string, string> = {
  power_lost: "#ef4444",
  power_restored: "#22c55e",
  boot: "#f59e0b",
  heartbeat: "#3b82f6",
}

const PULSE_DURATION = 3000

// ── Geometry helpers ──

function pointToSegDist(
  px: number,
  py: number,
  x1: number,
  y1: number,
  x2: number,
  y2: number,
): number {
  const dx = x2 - x1
  const dy = y2 - y1
  const lenSq = dx * dx + dy * dy
  if (lenSq === 0) return Math.hypot(px - x1, py - y1)
  let t = ((px - x1) * dx + (py - y1) * dy) / lenSq
  t = Math.max(0, Math.min(1, t))
  return Math.hypot(px - (x1 + t * dx), py - (y1 + t * dy))
}

// ── Draw functions ──

function drawEdge(
  ctx: CanvasRenderingContext2D,
  link: PositionedLink,
  selected: boolean,
) {
  ctx.beginPath()
  ctx.moveTo(link.x1, link.y1)
  ctx.lineTo(link.x2, link.y2)

  if (selected) {
    ctx.strokeStyle = COLORS.edgeSelected
    ctx.lineWidth = 2.5
    ctx.setLineDash([])
  } else if (link.known) {
    ctx.strokeStyle = COLORS.edgeKnown
    ctx.lineWidth = 1.2
    ctx.setLineDash([])
  } else {
    ctx.strokeStyle = COLORS.edgeDashed
    ctx.lineWidth = 1.2
    ctx.setLineDash([6, 4])
  }
  ctx.stroke()
  ctx.setLineDash([])
}

function drawNode(
  ctx: CanvasRenderingContext2D,
  node: PositionedNode,
  selected: boolean,
  poleStates?: Map<string, PoleState>,
) {
  const isSelected = selected

  if (node.type === "pole") {
    const state = poleStates?.get(node.id)
    drawPole(ctx, node, isSelected, state?.energized, state?.reported)
  } else if (node.type === "root") {
    // virtual root — tiny dot, not interactive
    ctx.beginPath()
    ctx.arc(node.x, node.y, 3, 0, Math.PI * 2)
    ctx.fillStyle = COLORS.textMuted
    ctx.fill()
  } else {
    drawContainer(ctx, node, isSelected)
  }
}

function drawPole(
  ctx: CanvasRenderingContext2D,
  node: PositionedNode,
  selected: boolean,
  energized?: boolean,
  reported?: boolean,
) {
  const r = NODE_RADIUS

  if (selected) {
    ctx.beginPath()
    ctx.arc(node.x, node.y, r + 4, 0, Math.PI * 2)
    ctx.strokeStyle = COLORS.nodeSelected
    ctx.lineWidth = 2
    ctx.stroke()
  }

  ctx.beginPath()

  if (node.hasDevice) {
    ctx.arc(node.x, node.y, r, 0, Math.PI * 2)

    // 4 visual states:
    // - energized=true (or undefined with no event): white fill (healthy)
    // - energized=false, reported=true: dark red fill (device reported power_lost)
    // - energized=false, reported=false: amber fill (silent failure — device didn't report)
    // - energized=false, reported=undefined: dark red fill (unknown, treat as reported)
    let fillColor: string
    if (energized === false && reported === false) {
      fillColor = COLORS.poleSilentFail
    } else if (energized === false) {
      fillColor = COLORS.poleDark
    } else {
      fillColor = COLORS.poleFill
    }

    ctx.fillStyle = fillColor
    ctx.fill()
    ctx.strokeStyle = COLORS.nodeStroke
    ctx.lineWidth = 1
    ctx.stroke()
  } else {
    // Hollow dashed circle
    ctx.arc(node.x, node.y, r, 0, Math.PI * 2)
    ctx.fillStyle = COLORS.poleNoDevice
    ctx.fill()
    ctx.strokeStyle = COLORS.textMuted
    ctx.lineWidth = 1.5
    ctx.setLineDash([3, 3])
    ctx.stroke()
    ctx.setLineDash([])
  }
}

function drawContainer(
  ctx: CanvasRenderingContext2D,
  node: PositionedNode,
  selected: boolean,
) {
  const w = CONTAINER_W
  const h = CONTAINER_H
  const x = node.x - w / 2
  const y = node.y - h / 2
  const r = 4

  // Selection ring
  if (selected) {
    ctx.beginPath()
    ctx.roundRect(x - 3, y - 3, w + 6, h + 6, r + 2)
    ctx.strokeStyle = COLORS.nodeSelected
    ctx.lineWidth = 2
    ctx.stroke()
  }

  // Background
  ctx.beginPath()
  ctx.roundRect(x, y, w, h, r)
  const fill =
    node.type === "substation"
      ? COLORS.substationFill
      : node.type === "feeder"
        ? COLORS.feederFill
        : COLORS.dtFill
  ctx.fillStyle = fill
  ctx.fill()
  ctx.strokeStyle = selected ? COLORS.nodeSelected : COLORS.nodeStroke
  ctx.lineWidth = selected ? 1.5 : 1
  ctx.stroke()

  // Type badge (left side)
  const badge = node.type === "substation" ? "SUB" : node.type === "feeder" ? "FDR" : "DT"
  ctx.font = "bold 8px 'JetBrains Mono Variable', monospace"
  ctx.fillStyle = COLORS.textMuted
  ctx.textAlign = "left"
  ctx.textBaseline = "middle"
  ctx.fillText(badge, x + 6, y + h / 2)

  // Label
  ctx.font = "11px 'JetBrains Mono Variable', monospace"
  ctx.fillStyle = COLORS.text
  ctx.textAlign = "left"
  ctx.textBaseline = "middle"
  ctx.fillText(node.name, x + 34, y + h / 2)

  // Downstream count badge (right side)
  if (node.downstreamCount != null) {
    const countStr = String(node.downstreamCount)
    ctx.font = "9px 'JetBrains Mono Variable', monospace"
    const tw = ctx.measureText(countStr).width
    const bx = x + w - tw - 14
    const by = y + h / 2

    ctx.beginPath()
    ctx.arc(bx - 4, by, 8, 0, Math.PI * 2)
    ctx.fillStyle = "#333"
    ctx.fill()

    ctx.fillStyle = COLORS.textMuted
    ctx.textAlign = "center"
    ctx.fillText(countStr, bx - 4, by + 0.5)
  }

  // Expand/collapse indicator
  if (node.hasChildren) {
    const ix = x + w - 10
    const iy = node.y

    ctx.beginPath()
    if (node.collapsed) {
      // Filled right-pointing triangle (collapsed)
      ctx.moveTo(ix - INDICATOR_SIZE / 2, iy - INDICATOR_SIZE / 2)
      ctx.lineTo(ix + INDICATOR_SIZE / 2, iy)
      ctx.lineTo(ix - INDICATOR_SIZE / 2, iy + INDICATOR_SIZE / 2)
    } else {
      // Open down-pointing triangle (expanded)
      ctx.moveTo(ix - INDICATOR_SIZE / 2, iy - INDICATOR_SIZE / 2 + 2)
      ctx.lineTo(ix + INDICATOR_SIZE / 2, iy - INDICATOR_SIZE / 2 + 2)
      ctx.lineTo(ix, iy + INDICATOR_SIZE / 2)
    }
    ctx.closePath()
    ctx.fillStyle = COLORS.indicatorFill
    ctx.fill()
  }
}

// ── Hit detection ──

function hitTestNode(
  nodes: PositionedNode[],
  mx: number,
  my: number,
): PositionedNode | null {
  // Check in reverse order (topmost first)
  for (let i = nodes.length - 1; i >= 0; i--) {
    const n = nodes[i]
    if (n.type === "root") continue

    if (n.type === "pole") {
      const dist = Math.hypot(mx - n.x, my - n.y)
      if (dist <= NODE_RADIUS + 4) return n
    } else {
      // Container — check if inside expanded indicator zone
      const w = CONTAINER_W
      const h = CONTAINER_H
      const lx = n.x - w / 2
      const ly = n.y - h / 2

      // Check indicator click first (if has children)
      if (n.hasChildren) {
        const ix = lx + w - 10
        const iy = n.y
        if (Math.hypot(mx - ix, iy - my) <= INDICATOR_SIZE) {
          return n // will be handled as toggle
        }
      }

      // Check container body
      if (mx >= lx && mx <= lx + w && my >= ly && my <= ly + h) {
        return n
      }
    }
  }
  return null
}

function hitTestEdge(
  links: PositionedLink[],
  mx: number,
  my: number,
): PositionedLink | null {
  const threshold = 14
  for (let i = links.length - 1; i >= 0; i--) {
    const l = links[i]
    const dist = pointToSegDist(mx, my, l.x1, l.y1, l.x2, l.y2)
    if (dist <= threshold) return l
  }
  return null
}

function isInIndicatorZone(
  node: PositionedNode,
  mx: number,
  my: number,
): boolean {
  if (!node.hasChildren || node.type === "pole") return false
  const w = CONTAINER_W
  const ix = node.x - w / 2 + w - 10
  const iy = node.y
  return Math.hypot(mx - ix, iy - my) <= INDICATOR_SIZE + 2
}

function isSelected(selection: CanvasSelection[], type: "node" | "edge", id: string): boolean {
  return selection.some((s) => s.type === type && s.id === id)
}

function drawFaultMarker(
  ctx: CanvasRenderingContext2D,
  faultTarget: string,
  links: PositionedLink[],
) {
  // faultTarget format: "P-1->P-2" for span, or "D-123" for DT, or "F-001" for feeder
  if (!faultTarget.includes("->")) return // DT/feeder faults don't have a single edge to mark

  const [sourceId, targetId] = faultTarget.split("->")

  // Find the link in the current links
  const link = links.find((l) => l.sourceId === sourceId && l.targetId === targetId)
  if (!link) return

  const midX = (link.x1 + link.x2) / 2
  const midY = (link.y1 + link.y2) / 2

  // Draw red X marker
  const size = 8
  ctx.beginPath()
  ctx.moveTo(midX - size, midY - size)
  ctx.lineTo(midX + size, midY + size)
  ctx.moveTo(midX + size, midY - size)
  ctx.lineTo(midX - size, midY + size)
  ctx.strokeStyle = "#ef4444"
  ctx.lineWidth = 2.5
  ctx.stroke()

  // Also highlight the edge
  ctx.beginPath()
  ctx.moveTo(link.x1, link.y1)
  ctx.lineTo(link.x2, link.y2)
  ctx.strokeStyle = "#ef4444"
  ctx.lineWidth = 3
  ctx.stroke()
}

// ── Component ──

export function NetworkCanvas({
  nodes,
  links,
  selection,
  transform,
  onSelect,
  onToggleCollapse,
  onTransformChange,
  poleStates,
  lastEvents,
  activeFaults,
}: NetworkCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const transformRef = useRef(transform ?? { x: 0, y: 0, k: 1 })
  const dimsRef = useRef({ width: 0, height: 0 })
  const nodesRef = useRef(nodes)
  const linksRef = useRef(links)
  const selectionRef = useRef(selection)
  const poleStatesRef = useRef(poleStates)
  const activeFaultsRef = useRef(activeFaults)
  const nodeIndexRef = useRef(new Map<string, PositionedNode>())
  const activePulsesRef = useRef<ActivePulse[]>([])
  const rafIdRef = useRef<number | null>(null)

  const [dragging, setDragging] = useState(false)
  const dragRef = useRef({ startX: 0, startY: 0, startTx: 0, startTy: 0 })
  const hasAutoFitRef = useRef(transform !== null)

  // Keep refs updated
  useEffect(() => { activeFaultsRef.current = activeFaults }, [activeFaults])

  // ── Canvas render (imperative, reads from refs) ──

  const doRender = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext("2d")
    if (!ctx) return
    const { width, height } = canvas
    const t = transformRef.current
    const { x, y, k } = t

    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = COLORS.bg
    ctx.fillRect(0, 0, width, height)

    ctx.save()
    ctx.translate(x, y)
    ctx.scale(k, k)

    for (const link of linksRef.current) {
      if (link.sourceId === "__root__" || link.targetId === "__root__") continue
      drawEdge(ctx, link, isSelected(selectionRef.current, "edge", `${link.sourceId}->${link.targetId}`))
    }

    // Draw fault markers on faulted edges
    const faults = activeFaultsRef.current
    if (faults) {
      for (const fault of faults) {
        drawFaultMarker(ctx, fault.target, linksRef.current)
      }
    }

    for (const node of nodesRef.current) {
      if (node.type === "root") continue
      drawNode(ctx, node, isSelected(selectionRef.current, "node", node.id), poleStatesRef.current)
    }

    ctx.restore()
  }, []) // no deps — reads from refs

  // ── Redraw when props change ──

  useEffect(() => {
    nodesRef.current = nodes
    linksRef.current = links
    doRender()
  }, [nodes, links, doRender])

  useEffect(() => {
    selectionRef.current = selection
    doRender()
  }, [selection, doRender])

  // ── Pole state tracking ──

  useEffect(() => {
    poleStatesRef.current = poleStates
    doRender()
  }, [poleStates, doRender])

  // ── Build node index for fast pole lookup ──

  useEffect(() => {
    const index = new Map<string, PositionedNode>()
    for (const node of nodes) {
      if (node.type === "pole") {
        index.set(node.id, node)
      }
    }
    nodeIndexRef.current = index
  }, [nodes])

  // ── Draw pulse overlays ──

  const drawPulseOverlay = useCallback((now: number) => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext("2d")
    if (!ctx) return
    const t = transformRef.current

    ctx.save()
    ctx.translate(t.x, t.y)
    ctx.scale(t.k, t.k)

    for (const pulse of activePulsesRef.current) {
      const node = nodeIndexRef.current.get(pulse.poleId)
      if (!node) continue

      const progress = (now - pulse.startTime) / pulse.duration
      const radius = NODE_RADIUS + 20 * progress
      const opacity = 0.8 * (1 - progress)

      ctx.beginPath()
      ctx.arc(node.x, node.y, radius, 0, Math.PI * 2)
      ctx.globalAlpha = opacity
      ctx.strokeStyle = PULSE_COLORS[pulse.event] ?? "#888"
      ctx.lineWidth = 2
      ctx.stroke()
    }

    ctx.globalAlpha = 1
    ctx.restore()
  }, [])

  // ── Pulse animation: add pulse on new event ──

  useEffect(() => {
    if (!lastEvents || lastEvents.length === 0) return

    for (const evt of lastEvents) {
      activePulsesRef.current.push({
        poleId: evt.pole_id,
        event: evt.event,
        startTime: performance.now(),
        duration: PULSE_DURATION,
      })
    }

    // Start rAF loop if not already running
    if (rafIdRef.current === null) {
      const tick = () => {
        const now = performance.now()

        // Remove expired pulses
        activePulsesRef.current = activePulsesRef.current.filter(
          (p) => now - p.startTime < p.duration,
        )

        if (activePulsesRef.current.length === 0) {
          rafIdRef.current = null
          doRender()
          return
        }

        doRender()
        drawPulseOverlay(now)

        rafIdRef.current = requestAnimationFrame(tick)
      }

      rafIdRef.current = requestAnimationFrame(tick)
    }
  }, [lastEvents, doRender, drawPulseOverlay])

  // ── Cleanup rAF on unmount ──

  useEffect(() => {
    return () => {
      if (rafIdRef.current !== null) {
        cancelAnimationFrame(rafIdRef.current)
      }
    }
  }, [])

  // ── Canvas resize ──

  useLayoutEffect(() => {
    const container = containerRef.current
    const canvas = canvasRef.current
    if (!container || !canvas) return

    const dpr = window.devicePixelRatio || 1
    const w = container.clientWidth
    const h = container.clientHeight

    dimsRef.current = { width: w, height: h }
    canvas.width = w * dpr
    canvas.height = h * dpr
    canvas.style.width = `${w}px`
    canvas.style.height = `${h}px`

    if (w === 0 || h === 0) return

    if (!hasAutoFitRef.current) {
      hasAutoFitRef.current = true
      const visible = nodesRef.current.filter((n) => n.type !== "root")
      if (visible.length === 0) return

      const PAD = 48
      const minX = Math.min(...visible.map((n) => n.x))
      const maxX = Math.max(...visible.map((n) => n.x))
      const minY = Math.min(...visible.map((n) => n.y))
      const maxY = Math.max(...visible.map((n) => n.y))

      const k = Math.min(w / (maxX - minX + PAD * 2), h / (maxY - minY + PAD * 2), 2)
      const x = w / 2 - ((minX + maxX) / 2) * k
      const y = h / 2 - ((minY + maxY) / 2) * k

      const next = { x, y, k }
      transformRef.current = next
      onTransformChange(next)
      doRender()
      return
    }

    doRender()
  }, [doRender, onTransformChange])

  // ── Mouse events ──

  const screenToWorld = useCallback(
    (sx: number, sy: number): [number, number] => {
      const t = transformRef.current
      return [(sx - t.x) / t.k, (sy - t.y) / t.k]
    },
    [],
  )

  const handleWheel = useCallback(
    (e: React.WheelEvent) => {
      e.preventDefault()
      const rect = canvasRef.current?.getBoundingClientRect()
      if (!rect) return

      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top
      const t = transformRef.current
      const delta = e.deltaY > 0 ? 0.9 : 1.1
      const newK = Math.max(0.1, Math.min(5, t.k * delta))
      const newX = mx - (mx - t.x) * (newK / t.k)
      const newY = my - (my - t.y) * (newK / t.k)

      const next = { x: newX, y: newY, k: newK }
      transformRef.current = next
      onTransformChange(next)
      doRender()
    },
    [doRender, onTransformChange],
  )

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      if (e.button !== 0) return
      e.preventDefault()
      const rect = canvasRef.current?.getBoundingClientRect()
      if (!rect) return

      const sx = e.clientX - rect.left
      const sy = e.clientY - rect.top
      const [wx, wy] = screenToWorld(sx, sy)

      const multiSelect = e.ctrlKey || e.metaKey

      const hitNode = hitTestNode(nodesRef.current, wx, wy)
      if (hitNode) {
        if (isInIndicatorZone(hitNode, wx, wy)) {
          onToggleCollapse(hitNode.id)
          return
        }
        onSelect([{ type: "node", id: hitNode.id }], multiSelect)
        return
      }

      const hitEdge = hitTestEdge(linksRef.current, wx, wy)
      if (hitEdge) {
        onSelect([{ type: "edge", id: `${hitEdge.sourceId}->${hitEdge.targetId}` }], multiSelect)
        return
      }

      onSelect([], false)
      const t = transformRef.current
      setDragging(true)
      dragRef.current = { startX: sx, startY: sy, startTx: t.x, startTy: t.y }
    },
    [onSelect, onToggleCollapse, screenToWorld],
  )

  const handleMouseMove = useCallback(
    (e: React.MouseEvent) => {
      if (!dragging) return
      const rect = canvasRef.current?.getBoundingClientRect()
      if (!rect) return

      const dx = e.clientX - rect.left - dragRef.current.startX
      const dy = e.clientY - rect.top - dragRef.current.startY
      const t = transformRef.current

      const next = { x: dragRef.current.startTx + dx, y: dragRef.current.startTy + dy, k: t.k }
      transformRef.current = next
      onTransformChange(next)
      doRender()
    },
    [dragging, doRender, onTransformChange],
  )

  const handleMouseUp = useCallback(() => {
    setDragging(false)
  }, [])

  // Cursor
  const cursor = dragging ? "grabbing" : "grab"

  return (
    <div ref={containerRef} className="h-full w-full overflow-hidden select-none">
      <canvas
        ref={canvasRef}
        style={{ cursor }}
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
      />
    </div>
  )
}
