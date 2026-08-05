import { useNavigate } from "react-router-dom"
import { useState } from "react"
import { useTickets } from "@/hooks/useTickets"
import { MapPin, Warning, Check, Spinner, DotsThree, ArrowRight } from "@phosphor-icons/react"
import { format } from "date-fns"
import type { Ticket } from "@/lib/types"

function SeverityBadge({ severity }: { severity: Ticket["severity"] }) {
  const styles = {
    critical: "bg-red-100 text-red-700",
    major: "bg-amber-100 text-amber-700",
    minor: "bg-blue-100 text-blue-700",
  }
  return <span className={`px-2 py-0.5 rounded text-xs font-medium ${styles[severity]}`}>{severity}</span>
}

function StatusBadge({ status }: { status: Ticket["status"] }) {
  const styles = {
    active: "bg-red-100 text-red-700",
    verified: "bg-green-100 text-green-700",
    resolved: "bg-gray-100 text-gray-700",
  }
  const icons = {
    active: Warning,
    verified: Check,
    resolved: Check,
  }
  const Icon = icons[status]
  return (
    <span className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${styles[status]}`}>
      <Icon className="size-3" weight="bold" />
      {status}
    </span>
  )
}

function ConfidenceBar({ confidence }: { confidence: number }) {
  const pct = Math.round(confidence * 100)
  const color = confidence >= 0.8 ? "bg-green-500" : confidence >= 0.5 ? "bg-amber-500" : "bg-red-500"
  return (
    <div className="w-24 h-2 bg-gray-200 rounded-full overflow-hidden">
      <div className={`${color} h-full transition-all`} style={{ width: `${pct}%` }} />
    </div>
  )
}

function TicketDetail({ 
  ticket, 
  onClose, 
  onAcknowledge, 
  onResolve,
  navigate
}: { 
  ticket: Ticket
  onClose: () => void
  onAcknowledge: (id: string) => Promise<void>
  onResolve: (id: string) => Promise<void>
  navigate: (path: string) => void
}) {
  const [ackLoading, setAckLoading] = useState(false)
  const [resolveLoading, setResolveLoading] = useState(false)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-3xl max-h-[80vh] overflow-y-auto rounded-lg bg-card shadow-xl">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h2 className="font-semibold text-lg">{ticket.id}</h2>
            <p className="text-sm text-muted-foreground">{ticket.dt_id} • {ticket.feeder_id}</p>
          </div>
          <button onClick={onClose} className="p-1 hover:bg-accent rounded">
            <DotsThree className="size-5" weight="bold" />
          </button>
        </div>

        <div className="p-6 space-y-6">
          <div className="grid grid-cols-3 gap-4">
            <div>
              <p className="text-xs text-muted-foreground">Severity</p>
              <SeverityBadge severity={ticket.severity} />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Status</p>
              <StatusBadge status={ticket.status} />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Confidence</p>
              <div className="flex items-center gap-2">
                <ConfidenceBar confidence={ticket.confidence} />
                <span className="text-sm font-mono">{Math.round(ticket.confidence * 100)}%</span>
              </div>
            </div>
          </div>

          <div className="border-t border-border pt-6">
            <h3 className="font-medium mb-3">Fault Details</h3>
            <dl className="grid grid-cols-2 gap-3 text-sm">
              <div><dt className="text-muted-foreground">Scope</dt><dd className="font-mono capitalize">{ticket.scope}</dd></div>
              <div><dt className="text-muted-foreground">Target</dt><dd className="font-mono">{ticket.target_id}</dd></div>
              <div><dt className="text-muted-foreground">DT</dt><dd className="font-mono">{ticket.dt_id}</dd></div>
              <div><dt className="text-muted-foreground">Feeder</dt><dd className="font-mono">{ticket.feeder_id}</dd></div>
              <div><dt className="text-muted-foreground">Affected Poles</dt><dd>{ticket.affected_count}</dd></div>
              <div><dt className="text-muted-foreground">PIN Code</dt><dd>{ticket.pincode || "—"}</dd></div>
            </dl>
          </div>

          <div className="border-t border-border pt-6">
            <h3 className="font-medium mb-3">Evidence Events</h3>
            <div className="max-h-48 overflow-y-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs text-muted-foreground uppercase">
                    <th className="pb-2">Time</th>
                    <th className="pb-2">Pole</th>
                    <th className="pb-2">Event</th>
                    <th className="pb-2">Energized</th>
                    <th className="pb-2">Reported</th>
                    <th className="pb-2">Seq</th>
                  </tr>
                </thead>
                <tbody>
                  {ticket.evidence.slice(0, 20).map((ev, i) => (
                    <tr key={i} className="border-t border-border/50">
                      <td className="py-1 font-mono">{format(new Date(ev.received_at), "HH:mm:ss")}</td>
                      <td className="py-1 font-mono">{ev.pole_id}</td>
                      <td className="py-1 capitalize">{ev.event}</td>
                      <td className="py-1">{ev.energized ? "✓" : "✗"}</td>
                      <td className="py-1">{ev.reported ? "✓" : "✗"}</td>
                      <td className="py-1 font-mono">{ev.seq}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-border pt-6">
            <button 
              onClick={() => { navigate(`/map?fault=${ticket.id}`); onClose(); }}
              className="px-4 py-2 border border-border rounded hover:bg-accent transition-colors"
            >
              <ArrowRight className="size-4 inline mr-1" />
              View on Map
            </button>
            {ticket.status === "active" && (
              <button
                onClick={async () => { setAckLoading(true); await onAcknowledge(ticket.id); setAckLoading(false) }}
                disabled={ackLoading}
                className="px-4 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90 transition-colors disabled:opacity-50"
              >
                {ackLoading ? "Acknowledging..." : "Acknowledge"}
              </button>
            )}
            {ticket.status === "verified" && (
              <button
                onClick={async () => { setResolveLoading(true); await onResolve(ticket.id); setResolveLoading(false) }}
                disabled={resolveLoading}
                className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors disabled:opacity-50"
              >
                {resolveLoading ? "Resolving..." : "Resolve"}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export function Dashboard() {
  const navigate = useNavigate()
  const { tickets, loading, error, acknowledge, resolve } = useTickets()
  const [selectedTicket, setSelectedTicket] = useState<Ticket | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const handleViewOnMap = (ticket: Ticket) => {
    navigate(`/map?fault=${ticket.id}`)
  }

  const handleRowClick = (ticket: Ticket) => {
    setSelectedTicket(ticket)
    setDetailOpen(true)
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="size-8 animate-spin text-primary" weight="bold" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center text-destructive">
        <p>Error loading tickets: {error.message}</p>
      </div>
    )
  }

  const sortedTickets = [...tickets].sort((a, b) => 
    new Date(b.detected_at).getTime() - new Date(a.detected_at).getTime()
  )

  return (
    <div className="flex h-full flex-col p-4 gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Active Faults</h1>
        <span className="px-3 py-1 bg-primary/10 text-primary rounded-full text-sm font-medium">
          {tickets.length} tickets
        </span>
      </div>

      <div className="flex-1 overflow-hidden rounded-lg border border-border bg-card">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-muted/50">
              <tr className="text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">Severity</th>
                <th className="px-4 py-3">Scope</th>
                <th className="px-4 py-3">Target</th>
                <th className="px-4 py-3">DT</th>
                <th className="px-4 py-3">Affected</th>
                <th className="px-4 py-3">Confidence</th>
                <th className="px-4 py-3">Detected</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3 w-12"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sortedTickets.map((ticket) => (
                <tr 
                  key={ticket.id} 
                  onClick={() => handleRowClick(ticket)}
                  className="cursor-pointer hover:bg-accent/50 transition-colors"
                >
                  <td className="px-4 py-3 font-mono text-sm">{ticket.id}</td>
                  <td className="px-4 py-3"><SeverityBadge severity={ticket.severity} /></td>
                  <td className="px-4 py-3 text-sm font-medium capitalize">{ticket.scope}</td>
                  <td className="px-4 py-3 font-mono text-sm">{ticket.target_id}</td>
                  <td className="px-4 py-3 font-mono text-sm">{ticket.dt_id}</td>
                  <td className="px-4 py-3 text-sm">{ticket.affected_count}</td>
                  <td className="px-4 py-3"><ConfidenceBar confidence={ticket.confidence} /></td>
                  <td className="px-4 py-3 text-sm text-muted-foreground">
                    {format(new Date(ticket.detected_at), "HH:mm:ss")}
                  </td>
                  <td className="px-4 py-3"><StatusBadge status={ticket.status} /></td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleViewOnMap(ticket) }}
                      className="p-1.5 hover:bg-accent rounded transition-colors"
                      title="View on Map"
                    >
                      <MapPin className="size-4 text-muted-foreground" />
                    </button>
                  </td>
                </tr>
              ))}
              {tickets.length === 0 && (
                <tr>
                  <td colSpan={10} className="px-4 py-8 text-center text-muted-foreground">
                    No active faults detected
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {detailOpen && selectedTicket && (
        <TicketDetail 
          ticket={selectedTicket} 
          onClose={() => setDetailOpen(false)} 
          onAcknowledge={acknowledge}
          onResolve={resolve}
          navigate={navigate}
        />
      )}
    </div>
  )
}