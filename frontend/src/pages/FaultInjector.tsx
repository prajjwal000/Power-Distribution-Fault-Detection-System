import { useCallback, useState } from "react"
import { useFaultInjector } from "@/hooks/useFaultInjector"
import { useSimulatorEvents } from "@/hooks/useSimulatorEvents"
import { useFaults } from "@/hooks/useFaults"
import { NetworkCanvas } from "@/components/fault-injector/NetworkCanvas"
import { Legend } from "@/components/fault-injector/Legend"
import { ClockDisplay } from "@/components/fault-injector/ClockDisplay"
import { Lightning, ArrowCounterClockwise, Trash } from "@phosphor-icons/react"

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
  } = useFaultInjector()

  const { poleStates, lastEvent, connected } = useSimulatorEvents()
  const { faults, loading, error, injectFault, repairFault, repairAll, refresh } = useFaults()

  const [injecting, setInjecting] = useState(false)

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
    const toExpand = new Set<string>()
    for (const pid of poleIds) {
      let current = pid
      while (current && collapsedIds.has(current)) {
        toExpand.add(current)
        const node = nodes.find((n) => n.id === current)
        if (!node || node.type === "pole") break
        // For containers, parent is in the tree hierarchy
        // This is simplified - the real parent would be found via data
        current = ""
      }
    }
    toExpand.forEach((id) => toggleCollapse(id))
  }, [nodes, collapsedIds, toggleCollapse])

  const handleInject = useCallback(async (target: { type: "span" | "dt" | "feeder"; parent_id?: string; child_id?: string; target_id?: string }) => {
    setInjecting(true)
    const fault = await injectFault(target)
    setInjecting(false)
    if (fault) {
      ensureVisible(fault.affected_poles)
    }
  }, [injectFault, ensureVisible])

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
          lastEvent={lastEvent}
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
              <div className="text-xs font-medium text-foreground mb-2">Inject Fault</div>

              {selectedEdges.map((edge) => (
                <button
                  key={edge.id}
                  onClick={() => {
                    const [parent_id, child_id] = edge.id.split("->")
                    handleInject({ type: "span", parent_id, child_id })
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
                      onClick={() => handleInject({ type: "dt", target_id: node.id })}
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
                      onClick={() => handleInject({ type: "feeder", target_id: node.id })}
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

        <div className="mt-4 text-[10px] text-muted-foreground border-t border-border pt-3 space-y-1">
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