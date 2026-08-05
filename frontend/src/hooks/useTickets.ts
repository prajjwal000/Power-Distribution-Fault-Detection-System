import { useEffect, useState, useCallback } from "react"
import type { Ticket } from "@/lib/types"

export interface TicketsState {
  tickets: Ticket[]
  loading: boolean
  error: Error | null
}

export function useTickets(): TicketsState & { 
  acknowledge: (id: string) => Promise<void>
  resolve: (id: string) => Promise<void>
} {
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchTickets = useCallback(async () => {
    try {
      const res = await fetch("/api/tickets")
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      setTickets(data)
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const eventSource = new EventSource("/api/tickets/stream")
    
    const loadTickets = async () => {
      try {
        await fetchTickets()
      } catch {
        // Ignore errors during initial load
      }
    }
    
    loadTickets()

    eventSource.onmessage = (event) => {
      try {
        const update = JSON.parse(event.data)
        if (update.type === "ticket_created" || update.type === "ticket_refined" || update.type === "ticket_verified") {
          setTickets(prev => {
            const exists = prev.find(t => t.id === update.ticket.id)
            if (exists) {
              return prev.map(t => t.id === update.ticket.id ? update.ticket : t)
            }
            return [update.ticket, ...prev]
          })
        }
      } catch {
        // Ignore parse errors
      }
    }
    eventSource.onerror = () => {
      eventSource.close()
    }

    return () => {
      eventSource.close()
    }
  }, [fetchTickets])

  const acknowledge = async (id: string) => {
    const res = await fetch(`/api/tickets/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status: "acknowledged" })
    })
    if (!res.ok) throw new Error("Failed to acknowledge")
    await fetchTickets()
  }

  const resolve = async (id: string) => {
    const res = await fetch(`/api/tickets/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status: "resolved" })
    })
    if (!res.ok) throw new Error("Failed to resolve")
    await fetchTickets()
  }

  return { tickets, loading, error, acknowledge, resolve }
}