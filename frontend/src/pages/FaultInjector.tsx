import { useCallback, useState } from "react"
import { useFaultInjector } from "@/hooks/useFaultInjector"
import { useSimulatorEvents } from "@/hooks/useSimulatorEvents"
import { useFaults } from "@/hooks/useFaults"
import { useNoise } from "@/hooks/useNoise"
import { NetworkCanvas } from "@/components/fault-injector/NetworkCanvas"
import { Legend } from "@/components/fault-injector/Legend"
import { ClockDisplay } from "@/components/fault-injector/ClockDisplay"
import { NoisePanel } from "@/components/fault-injector/NoisePanel"
import { Lightning, ArrowCounterClockwise, Trash, Timer } from "@phosphor-icons/react"

const AUTO_REPAIR_OPTIONS = [
  { label: "Never", value: 0 },
  { label: "10 sec", value: 10 },
  { label: "30 sec", value: 30 },
  { label: "1 min", value: 60 },
  { label: "5 min", value: 300 },
  { label: "10 min", value: 600 },
] as const

export function FaultInjector() {
  const {
    nodes,
    links,
    selection,
    transform,
    toggleCollapse,
    expandAll,
    collapseAll,
    select,
    updateTransform,
    collapsedIds,
    networkData,
  } = useFaultInjector()

  const { poleStates, lastEvents, connected } = useSimulatorEvents()
  const { faults, loading, error, injectFault, repairFault, repairAll, refresh } = useFaults()
  const { killDevice } = useNoise()

  const [injecting, setInjecting] = useState(false)
  const [autoRepairSecs, setAutoRepairSecs] = useState<number>(0)

  const handleCollapseAll = useCallback(() => {
    const ids: string[] = []
    for (const dt of nodes.filter((n) => n.type === "dt").map((n) => n.id)) ids.push(dt)
    for (const feeder of nodes.filter((n) => n.type === "feeder").map((n) => n.id)) ids.push(feeder)
    for (const sub of nodes.filter((n) => n.type === "substation").map((n) => n.id)) ids.push(sub)
    collapseAll(ids)
  }, [nodes, collapseAll])

  const selectedNodes = selection
    .filter((s) => s.type === "node")
    .map((s) => nodes.find((n) => n.id === s.id))
    .filter((n): n is (typeof nodes)[0] => n != null)

  const selectedEdges = selection.filter((s) => s.type === "edge")

  // Auto-expand ancestors of a faulted area
  const ensureVisible = useCallback((poleIds: string[]) => {
    if (!networkData) return
    const toExpand = new Set<string>()

    // Build parent map from gt_topology: pole -> parent pole (or DT)
    const poleParentMap = new Map<string, string | null>()
    for (const gt of networkData.gt_topology) {
      poleParentMap.set(gt.pole_id, gt.parent_id)
    }

    // Build DT -> feeder map
    const dtFeederMap = new Map<string, string>()
    for (const dt of networkData.transformers) {
      dtFeederMap.set(dt.id, dt.feeder_id)
    }

    // Build feeder -> substation map
    const feederSubstationMap = new Map<string, string>()
    for (const feeder of networkData.feeders) {
      feederSubstationMap.set(feeder.id, feeder.substation_id)
    }

    for (const pid of poleIds) {
      let current: string | null = pid
      while (current && collapsedIds.has(current)) {
        toExpand.add(current)
        const parent = poleParentMap.get(current)
        if (parent) {
          // Parent is another pole
          current = parent
        } else {
          // current is a root pole under a DT; find its DT
          const poleNode = networkData.gt_topology.find((gt) => gt.pole_id === current)
          if (poleNode) {
            current = poleNode.dt_id
          } else {
            current = null
          }
        }
      }
      // If we exited the loop because current was not collapsed, we may have reached a DT/feeder/substation
      // that IS collapsed - check and add it
      while (current && collapsedIds.has(current)) {
        toExpand.add(current)
        const feederId = dtFeederMap.get(current)
        if (feederId) {
          current = feederId
        } else {
          const substationId = feederSubstationMap.get(current)
          if (substationId) {
            current = substationId
          } else {
            current = null
          }
        }
      }
    }
    toExpand.forEach((id) => toggleCollapse(id))
  }, [networkData, collapsedIds, toggleCollapse])

  const handleInject = useCallback(async (target: { type: "span" | "dt" | "feeder"; parent_id?: string; child_id?: string; target_id?: string; auto_repair_secs?: number }) => {
    setInjecting(true)
    const fault = await injectFault(target)
    setInjecting(false)
    if (fault) {
      ensureVisible(fault.affected_poles)
    }
  }, [injectFault, ensureVisible])

  const handleInjectAll = useCallback(async () => {
    setInjecting(true)
    let lastFault: Awaited<ReturnType<typeof injectFault>> = null
    for (const edge of selectedEdges) {
      const [parent_id, child_id] = edge.id.split("->")
      const fault = await injectFault({ type: "span", parent_id, child_id, auto_repair_secs: autoRepairSecs })
      if (fault) lastFault = fault
    }
    for (const node of selectedNodes) {
      if (node.type === "dt") {
        const fault = await injectFault({ type: "dt", target_id: node.id, auto_repair_secs: autoRepairSecs })
        if (fault) lastFault = fault
      } else if (node.type === "feeder") {
        const fault = await injectFault({ type: "feeder", target_id: node.id, auto_repair_secs: autoRepairSecs })
        if (fault) lastFault = fault
      }
    }
    setInjecting(false)
    if (lastFault) {
      ensureVisible(lastFault.affected_poles)
    }
  }, [selectedEdges, selectedNodes, injectFault, ensureVisible, autoRepairSecs])

  const handleRepair = useCallback(async (id: string) => {
    await repairFault(id)
  }, [repairFault])

  const handleRepairAll = useCallback(async () => {
    await repairAll()
  }, [repairAll])

  return (
    <div className="flex h-full">
      {/* Canvas area */}
      <div className="relative flex-1 overflow-hidden">
        <ClockDisplay />

        <NetworkCanvas
          nodes={nodes}
          links={links}
          selection={selection}
          transform={transform}
          onSelect={select}
          onToggleCollapse={toggleCollapse}
          onTransformChange={updateTransform}
          poleStates={poleStates}
          lastEvents={lastEvents}
          activeFaults={faults}
        />
        <Legend />

        {/* Expand/collapse controls */}
        <div className="pointer-events-auto absolute right-3 top-3 flex gap-1">
          <button
            onClick={expandAll}
            className="rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-muted-foreground backdrop-blur-sm hover:bg-accent"
          >
            Expand all
          </button>
          <button
            onClick={handleCollapseAll}
            className="rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-muted-foreground backdrop-blur-sm hover:bg-accent"
          >
            Collapse all
          </button>
        </div>

        {/* Injection Stats overlay - bottom right */}
        {faults.length > 0 && (() => {
          const latestFault = [...faults].sort((a, b) => b.start_sim - a.start_sim)[0]
          if (!latestFault) return null
          const affectedPoles = latestFault.affected_poles || []
          const devicesOnPoles = affectedPoles.map((poleId) => 
            networkData?.registry_poles.find((p) => p.id === poleId)
          ).filter(Boolean)
          const totalDevices = devicesOnPoles.filter((p) => p?.device_id).length
          return (
            <div className="pointer-events-auto absolute bottom-3 right-3 w-64 rounded-md border border-border bg-card/95 p-2 backdrop-blur-sm">
              <div className="flex items-center gap-1.5 mb-1.5">
                <Timer className="size-3 text-green-500" weight="fill" />
                <span className="text-[10px] font-medium text-foreground">Last Injection</span>
              </div>
              <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[10px]">
                <div className="truncate text-muted-foreground">Target</div>
                <div className="font-mono text-foreground truncate">{latestFault.target}</div>
                <div className="text-muted-foreground">Poles</div>
                <div className="font-mono text-foreground">{latestFault.affected_count}</div>
                <div className="text-muted-foreground">Devices</div>
                <div className="font-mono text-foreground">{totalDevices}</div>
                <div className="text-muted-foreground">Auto-repair</div>
                <div className="font-mono text-foreground">
                  {latestFault.auto_repair_sim_secs && latestFault.auto_repair_sim_secs > 0
                    ? `${Math.round(latestFault.auto_repair_sim_secs / 30)}s`
                    : "Never"}
                </div>
              </div>
            </div>
          )
        })()}
      </div>

      {/* Right panel */}
      <div className="w-80 border-l border-border p-4 flex flex-col">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium">Injector</h2>
          <button
            onClick={refresh}
            disabled={loading}
            className="rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-muted-foreground backdrop-blur-sm hover:bg-accent"
          >
            <ArrowCounterClockwise className="size-3 inline" weight="duotone" />
          </button>
        </div>

        {error && (
          <div className="mt-2 rounded border border-destructive bg-destructive/10 p-2 text-[10px] text-destructive">
            {error.message}
          </div>
        )}

        <div className="mt-3 space-y-2 flex-1 overflow-y-auto">
          {/* Selection-based injection */}
          {(selectedNodes.length > 0 || selectedEdges.length > 0) && (
            <div className="rounded-md border border-border bg-card p-3">
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-medium text-foreground">Inject Fault</div>
                {(selectedEdges.length + selectedNodes.filter((n) => n.type === "dt" || n.type === "feeder").length) > 1 && (
                  <button
                    onClick={handleInjectAll}
                    disabled={injecting || loading}
                    className="rounded border border-yellow-500/50 bg-yellow-500/10 px-2 py-0.5 text-[10px] text-yellow-500 hover:bg-yellow-500/20 disabled:opacity-50 flex items-center gap-1"
                  >
                    <Lightning className="size-3" weight="fill" />
                    Inject All ({selectedEdges.length + selectedNodes.filter((n) => n.type === "dt" || n.type === "feeder").length})
                  </button>
                )}
              </div>

              {/* Auto-repair timer */}
              <div className="mb-2">
                <label className="block text-[10px] text-muted-foreground mb-1">Auto-repair after</label>
                <select
                  value={autoRepairSecs}
                  onChange={(e) => setAutoRepairSecs(Number(e.target.value))}
                  disabled={injecting || loading}
                  className="w-full rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] text-foreground hover:bg-accent disabled:opacity-50"
                >
                  {AUTO_REPAIR_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              {selectedEdges.map((edge) => (
                <button
                  key={edge.id}
                  onClick={() => {
                    const [parent_id, child_id] = edge.id.split("->")
                    handleInject({ type: "span", parent_id, child_id, auto_repair_secs: autoRepairSecs })
                  }}
                  disabled={injecting || loading}
                  className="w-full rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] text-foreground hover:bg-accent disabled:opacity-50 flex items-center gap-2"
                >
                  <Lightning className="size-3.5 text-yellow-500" weight="fill" />
                  <span>Inject span fault on <strong>{edge.id}</strong></span>
                </button>
              ))}

              {selectedNodes.map((node) => {
                if (node.type === "dt") {
                  return (
                    <button
                      key={node.id}
                      onClick={() => handleInject({ type: "dt", target_id: node.id, auto_repair_secs: autoRepairSecs })}
                      disabled={injecting || loading}
                      className="w-full rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] text-foreground hover:bg-accent disabled:opacity-50 flex items-center gap-2"
                    >
                      <Lightning className="size-3.5 text-yellow-500" weight="fill" />
                      <span>Inject DT fault on <strong>{node.name}</strong></span>
                    </button>
                  )
                }
                if (node.type === "feeder") {
                  return (
                    <button
                      key={node.id}
                      onClick={() => handleInject({ type: "feeder", target_id: node.id, auto_repair_secs: autoRepairSecs })}
                      disabled={injecting || loading}
                      className="w-full rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] text-foreground hover:bg-accent disabled:opacity-50 flex items-center gap-2"
                    >
                      <Lightning className="size-3.5 text-yellow-500" weight="fill" />
                      <span>Inject feeder fault on <strong>{node.name}</strong></span>
                    </button>
                  )
                }
                return null
              })}
            </div>
          )}

          {/* Kill Telemetry for selected nodes with devices */}
          {selectedNodes.some((n) => n.hasDevice === true) && (
            <div className="rounded-md border border-border bg-card p-3">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Lightning className="size-3.5 text-red-500" weight="fill" />
                  <div className="text-xs font-medium text-foreground">Kill Telemetry</div>
                </div>
                <span className="text-[10px] text-muted-foreground">
                  {autoRepairSecs === 0 ? "No auto-resume" : `Auto-resumes in ${autoRepairSecs}s`}
                </span>
              </div>
              <div className="space-y-1">
                {selectedNodes.filter((n) => n.hasDevice === true).map((node) => {
                  const deviceId = networkData?.registry_poles.find((p) => p.id === node.id)?.device_id
                  return deviceId ? (
                    <div key={node.id} className="flex items-center gap-2">
                      <span className="flex-1 text-[10px] text-muted-foreground truncate">{node.name}</span>
                      <button
                        onClick={() => killDevice(deviceId, autoRepairSecs > 0 ? autoRepairSecs : undefined)}
                        disabled={injecting || loading}
                        className="rounded border border-red-500/50 bg-red-500/10 px-2 py-1 text-[10px] text-red-500 hover:bg-red-500/20 disabled:opacity-50 flex items-center gap-1"
                      >
                        <Lightning className="size-3" weight="fill" />
                        Kill
                      </button>
                    </div>
                  ) : null
                })}
              </div>
            </div>
          )}

          {/* Active faults list */}
          {faults.length > 0 && (
            <div className="rounded-md border border-border bg-card p-3">
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-medium text-foreground">Active Faults ({faults.length})</div>
                <button
                  onClick={handleRepairAll}
                  disabled={loading}
                  className="rounded border border-destructive/50 bg-destructive/10 px-2 py-1 text-[10px] text-destructive hover:bg-destructive/20 disabled:opacity-50 flex items-center gap-1"
                >
                  <Trash className="size-3" weight="fill" />
                  Repair All
                </button>
              </div>

              <div className="space-y-1 max-h-48 overflow-y-auto">
                {faults.map((fault) => (
                  <div key={fault.id} className="rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] flex items-center justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <div className="font-mono text-foreground">{fault.target}</div>
                      <div className="text-muted-foreground">
                        {fault.type} &middot; {fault.affected_count} poles affected
                      </div>
                    </div>
                    <button
                      onClick={() => handleRepair(fault.id)}
                      disabled={loading}
                      className="rounded border border-border bg-card/90 px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-accent disabled:opacity-50 flex items-center gap-1"
                    >
                      <ArrowCounterClockwise className="size-3" weight="duotone" />
                      Repair
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Node/Edge inspection */}
          {selectedNodes.map((node) => (
            <div key={node.id} className="rounded-md border border-border bg-card p-3">
              <div className="text-xs font-medium text-foreground">{node.name}</div>
              <div className="mt-1 text-[10px] text-muted-foreground">
                Type: {node.type}
                {node.downstreamCount != null && (
                  <> &middot; {node.downstreamCount} downstream</>
                )}
                {node.hasDevice === true && <> &middot; Device OK</>}
                {node.hasDevice === false && <> &middot; No device</>}
                {node.hasTopology === false && (
                  <span className="text-yellow-500"> &middot; Topology unknown</span>
                )}
              </div>
            </div>
          ))}

          {selectedEdges.map((edge) => (
            <div key={edge.id} className="rounded-md border border-border bg-card p-3">
              <div className="text-xs font-medium text-foreground">{edge.id}</div>
              <div className="mt-1 text-[10px] text-muted-foreground">Type: edge</div>
            </div>
          ))}

          {selection.length === 0 && faults.length === 0 && (
            <p className="mt-3 text-xs text-muted-foreground">
              Click a node or edge to inspect it. Ctrl+click to multi-select.
              <br />
              Select an edge to inject a span fault, or a DT/feeder to inject that type.
            </p>
          )}
        </div>

        {/* Noise Panel - fixed at bottom */}
        <div className="mt-auto pt-2">
          <NoisePanel />
        </div>

        <div className="mt-2 text-[10px] text-muted-foreground border-t border-border pt-3 space-y-1">
          <div>Nodes: {nodes.length} &middot; Edges: {links.length}</div>
          <div className="flex items-center gap-1.5">
            <span className={`inline-block size-1.5 rounded-full ${connected ? "bg-green-500" : "bg-red-500"}`} />
            {connected ? "Live" : "Disconnected"}
          </div>
          {loading && <div className="text-yellow-500">Loading...</div>}
        </div>
      </div>
    </div>
  )
}