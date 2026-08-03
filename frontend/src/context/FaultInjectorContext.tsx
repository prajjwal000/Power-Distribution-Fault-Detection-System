import { createContext, useCallback, useState, useEffect, useMemo, type ReactNode } from "react"
import type { PositionedNode, PositionedLink } from "@/lib/treeLayout"
import type { NetworkData } from "@/lib/types"
import type { CanvasSelection } from "@/components/fault-injector/NetworkCanvas"

const COLLAPSED_STORAGE_KEY = "faultInjector_collapsedIds"
const TRANSFORM_STORAGE_KEY = "faultInjector_transform"

function loadCollapsedIds(): Set<string> {
  try {
    const raw = sessionStorage.getItem(COLLAPSED_STORAGE_KEY)
    if (!raw) return new Set()
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr)) return new Set()
    return new Set(arr)
  } catch {
    return new Set()
  }
}

function loadTransform(): { x: number; y: number; k: number } | null {
  try {
    const raw = sessionStorage.getItem(TRANSFORM_STORAGE_KEY)
    if (!raw) return null
    const t = JSON.parse(raw)
    if (typeof t.x === "number" && typeof t.y === "number" && typeof t.k === "number") {
      return t
    }
  } catch {
    // ignore
  }
  return null
}

// ── Context types ──

interface FaultInjectorContextValue {
  nodes: PositionedNode[]
  links: PositionedLink[]
  collapsedIds: Set<string>
  selection: CanvasSelection[]
  transform: { x: number; y: number; k: number } | null
  toggleCollapse: (id: string) => void
  expandAll: () => void
  collapseAll: (ids: string[]) => void
  select: (sel: CanvasSelection[], multiSelect: boolean) => void
  updateTransform: (t: { x: number; y: number; k: number }) => void
}

// eslint-disable-next-line react-refresh/only-export-components -- context must be in the same file as its provider
export const FaultInjectorContext = createContext<FaultInjectorContextValue | null>(null)

// ── Provider ──

interface FaultInjectorProviderProps {
  children: ReactNode
  data: NetworkData
  initialCollapsedIds?: string[]
}

export function FaultInjectorProvider({ children, data, initialCollapsedIds }: FaultInjectorProviderProps) {
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(() => {
    const stored = loadCollapsedIds()
    if (stored.size > 0) return stored
    if (initialCollapsedIds) return new Set(initialCollapsedIds)
    const ids = new Set<string>()
    for (const dt of data.transformers) ids.add(dt.id)
    return ids
  })

  const [selection, setSelection] = useState<CanvasSelection[]>([])
  const [transform, setTransform] = useState<{ x: number; y: number; k: number } | null>(() => loadTransform())

  // Persist collapsedIds
  useEffect(() => {
    sessionStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify([...collapsedIds]))
  }, [collapsedIds])

  // Persist transform
  useEffect(() => {
    if (transform === null) return
    sessionStorage.setItem(TRANSFORM_STORAGE_KEY, JSON.stringify(transform))
  }, [transform])

  const toggleCollapse = useCallback((id: string) => {
    setCollapsedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const expandAll = useCallback(() => {
    setCollapsedIds(new Set())
  }, [])

  const collapseAll = useCallback((ids: string[]) => {
    setCollapsedIds(new Set(ids))
  }, [])

  const select = useCallback((sel: CanvasSelection[], multiSelect: boolean) => {
    setSelection((prev) => {
      if (!multiSelect) return sel
      const next = [...prev]
      const idx = next.findIndex((s) => s.id === sel[0]?.id)
      if (idx >= 0) next.splice(idx, 1)
      else next.push(...sel)
      return next
    })
  }, [])

  const updateTransform = useCallback((t: { x: number; y: number; k: number }) => {
    setTransform(t)
  }, [])

  // Compute layout from data + collapsedIds
  const { nodes, links } = useMemo(() => {
    return computeLayoutFromData(data, collapsedIds)
  }, [data, collapsedIds])

  const value: FaultInjectorContextValue = {
    nodes,
    links,
    collapsedIds,
    selection,
    transform,
    toggleCollapse,
    expandAll,
    collapseAll,
    select,
    updateTransform,
  }

  return (
    <FaultInjectorContext.Provider value={value}>
      {children}
    </FaultInjectorContext.Provider>
  )
}

// ── Layout helper ──

import { computeLayout } from "@/lib/treeLayout"

function computeLayoutFromData(data: NetworkData, collapsedIds: Set<string>): {
  nodes: PositionedNode[]
  links: PositionedLink[]
} {
  return computeLayout(data, collapsedIds)
}
