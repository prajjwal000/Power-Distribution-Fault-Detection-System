import { useCallback } from "react"
import { useFaultInjector } from "@/hooks/useFaultInjector"
import { NetworkCanvas } from "@/components/fault-injector/NetworkCanvas"
import { Legend } from "@/components/fault-injector/Legend"

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
  } = useFaultInjector()

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

  return (
    <div className="flex h-full">
      {/* Canvas area */}
      <div className="relative flex-1 overflow-hidden">
        <NetworkCanvas
          nodes={nodes}
          links={links}
          selection={selection}
          transform={transform}
          onSelect={select}
          onToggleCollapse={toggleCollapse}
          onTransformChange={updateTransform}
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
      <div className="w-80 border-l border-border p-4">
        <h2 className="text-sm font-medium">Injector</h2>

        <div className="mt-3 space-y-2">
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
        </div>

        {selection.length === 0 && (
          <p className="mt-3 text-xs text-muted-foreground">
            Click a node or edge to inspect it. Ctrl+click to multi-select.
          </p>
        )}

        <div className="mt-4 text-[10px] text-muted-foreground">
          Nodes: {nodes.length} &middot; Edges: {links.length}
        </div>
      </div>
    </div>
  )
}
