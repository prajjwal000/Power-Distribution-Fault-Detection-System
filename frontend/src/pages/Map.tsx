import { useEffect, useState, useRef } from "react"
import { MapContainer, TileLayer, Marker, Popup, Polyline, CircleMarker, LayerGroup } from "react-leaflet"
import "leaflet/dist/leaflet.css"
import L from "leaflet"
import { useTopology } from "@/hooks/useTopology"
import { useTickets } from "@/hooks/useTickets"
import { useSearchParams, useNavigate } from "react-router-dom"
import { Lightning, House, Circle, CaretLeft, CaretRight } from "@phosphor-icons/react"
import type { Ticket } from "@/lib/types"

// Fix leaflet default icon issue
delete (L.Icon.Default.prototype as any)._getIconUrl
L.Icon.Default.mergeOptions({
  iconRetinaUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png",
  iconUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png",
  shadowUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png",
})

const BANGALORE_CENTER: [number, number] = [12.9716, 77.5946]

const SUBSTATION_ICON = L.divIcon({
  className: "custom-div-icon",
  html: `<div style="width: 24px; height: 24px; background: #3b82f6; border: 3px solid white; border-radius: 4px; transform: rotate(45deg); box-shadow: 0 2px 4px rgba(0,0,0,0.3);"></div>`,
  iconSize: [24, 24],
  iconAnchor: [12, 12],
})

const FEEDER_STYLE = { color: "#3b82f6", weight: 2, opacity: 0.6, dashArray: "5, 5" }
const KNOWN_EDGE_STYLE = { color: "#22c55e", weight: 2, opacity: 0.8 }
const INFERRED_EDGE_STYLE = { color: "#f59e0b", weight: 1.5, opacity: 0.6, dashArray: "3, 3" }

function PoleMarker({ pole, isDark, onClick }: { pole: any; isDark: boolean; onClick: () => void }) {
  const color = isDark ? "#ef4444" : "#22c55e"
  return (
    <CircleMarker
      center={[pole.lat, pole.lon]}
      radius={5}
      pathOptions={{
        color: "#fff",
        weight: 1.5,
        fillColor: color,
        fillOpacity: 0.9,
        interactive: true,
      }}
      eventHandlers={{
        click: onClick,
      }}
    >
      <Popup>{pole.id} - {isDark ? "DARK" : "ENERGIZED"}</Popup>
    </CircleMarker>
  )
}

function DTMarker({ dt, onClick }: { dt: any; onClick: () => void }) {
  return (
    <Marker position={[dt.lat, dt.lon]} icon={L.divIcon({
      className: "custom-div-icon",
      html: `<div style="width: 20px; height: 20px; background: #f59e0b; border: 2px solid white; border-radius: 50%; box-shadow: 0 2px 4px rgba(0,0,0,0.3); display: flex; align-items: center; justify-content: center;"><span style="color: white; font-size: 10px; font-weight: bold;">⚡</span></div>`,
      iconSize: [20, 20],
      iconAnchor: [10, 10],
    })} eventHandlers={{ click: onClick }}>
      <Popup>{dt.id} - {dt.capacity_kva} kVA</Popup>
    </Marker>
  )
}

function SubstationMarker({ sub, onClick }: { sub: any; onClick: () => void }) {
  return (
    <Marker position={[sub.lat, sub.lon]} icon={SUBSTATION_ICON} eventHandlers={{ click: onClick }}>
      <Popup>{sub.id} - Substation</Popup>
    </Marker>
  )
}

export function MapPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { data: topology, loading: topoLoading } = useTopology()
  const { tickets } = useTickets()
  
  const mapContainerRef = useRef<any>(null)
  const [selectedAsset, setSelectedAsset] = useState<any>(null)
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [showFaults, setShowFaults] = useState(true)
  const [showInferred, setShowInferred] = useState(true)
  const [showKnown, setShowKnown] = useState(true)
  const [showPoles, setShowPoles] = useState(true)
  
  const faultIdParam = searchParams.get("fault")
  const [highlightedFault, setHighlightedFault] = useState<Ticket | null>(null)

  const getMap = () => mapContainerRef.current?.leafletElement

  // Find fault from URL param
  useEffect(() => {
    if (faultIdParam) {
      const fault = tickets.find(t => t.id === faultIdParam)
      setHighlightedFault(fault || null)
      const m = getMap()
      if (fault?.location && m) {
        m.flyTo([fault.location.lat, fault.location.lon], 16)
      }
    }
  }, [faultIdParam, tickets])

  // Build feeder lines
  const feederLines = topology ? topology.feeders.flatMap(feeder => {
    const sub = topology.substations.find(s => s.id === feeder.substation_id)
    const dts = topology.transformers.filter(t => t.feeder_id === feeder.id)
    if (!sub || dts.length === 0) return []
    return dts.map(dt => ({
      positions: [[sub.lat, sub.lon], [dt.lat, dt.lon]] as [number, number][],
      feederId: feeder.id,
      dtId: dt.id,
    }))
  }) : []

  interface EdgeData {
  positions: [number, number][]
  childId: string
  parentId: string
  dtId: string
}

  // Build known edges (from registry topology)
  const knownEdges: EdgeData[] = topology ? topology.registry_poles
    .filter(p => p.parent_pole_id)
    .map(p => {
      const parent = topology.registry_poles.find(pp => pp.id === p.parent_pole_id)
      if (!parent) return null
      return {
        positions: [[parent.lat, parent.lon], [p.lat, p.lon]] as [number, number][],
        childId: p.id,
        parentId: parent.id,
        dtId: p.dt_id,
      }
    }).filter((e): e is EdgeData => e !== null) : []

  // Build inferred edges (placeholder - would come from API)
  const inferredEdges: any[] = []

  // Get dark poles from tickets
  const darkPoles = new Set(tickets.flatMap(t => t.affected_poles))

  if (topoLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    )
  }

  return (
    <div className="flex h-full">
      {/* Map */}
      <div className="flex-1 relative">
        <MapContainer
          ref={mapContainerRef}
          center={BANGALORE_CENTER}
          zoom={13}
          className="w-full h-full"
          scrollWheelZoom={true}
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />

          {/* Feeder lines */}
          <LayerGroup>
            {feederLines.map((line, i) => (
              <Polyline key={i} positions={line.positions} pathOptions={FEEDER_STYLE}>
                <Popup>Feeder: {line.feederId} → {line.dtId}</Popup>
              </Polyline>
            ))}
          </LayerGroup>

          {/* Known topology edges */}
          {showKnown && (
            <LayerGroup>
              {knownEdges.map((edge, i) => (
                <Polyline key={i} positions={edge.positions} pathOptions={KNOWN_EDGE_STYLE}>
                  <Popup>Known: {edge.parentId} → {edge.childId}</Popup>
                </Polyline>
              ))}
            </LayerGroup>
          )}

          {/* Inferred edges */}
          {showInferred && (
            <LayerGroup>
              {inferredEdges.map((edge, i) => (
                <Polyline key={i} positions={edge.positions} pathOptions={INFERRED_EDGE_STYLE}>
                  <Popup>Inferred: {edge.parentId} → {edge.childId}</Popup>
                </Polyline>
              ))}
            </LayerGroup>
          )}

          {/* Substations */}
          <LayerGroup>
            {topology?.substations.map(sub => (
              <SubstationMarker key={sub.id} sub={sub} onClick={() => setSelectedAsset(sub)} />
            ))}
          </LayerGroup>

          {/* DTs */}
          <LayerGroup>
            {topology?.transformers.map(dt => (
              <DTMarker key={dt.id} dt={dt} onClick={() => setSelectedAsset(dt)} />
            ))}
          </LayerGroup>

          {/* Poles */}
          {showPoles && topology && (
            <LayerGroup>
              {topology.registry_poles.map(pole => (
                <PoleMarker 
                  key={pole.id} 
                  pole={pole} 
                  isDark={darkPoles.has(pole.id)} 
                  onClick={() => setSelectedAsset(pole)}
                />
              ))}
            </LayerGroup>
          )}

          {/* Fault highlights */}
          {showFaults && tickets.map(ticket => (
            ticket.location && ticket.affected_poles.length > 0 && (
              <LayerGroup key={ticket.id}>
                <CircleMarker
                  center={[ticket.location.lat, ticket.location.lon]}
                  radius={15}
                  pathOptions={{
                    color: "#ef4444",
                    weight: 2,
                    fillColor: "#ef4444",
                    fillOpacity: 0.15,
                    dashArray: "5, 5",
                  }}
                >
                  <Popup>
                    <strong>{ticket.id}</strong><br/>
                    {ticket.target_id}<br/>
                    Confidence: {Math.round(ticket.confidence * 100)}%
                  </Popup>
                </CircleMarker>
              </LayerGroup>
            )
          ))}

          {/* Highlighted fault from URL */}
          {highlightedFault && highlightedFault.location && (
            <CircleMarker
              center={[highlightedFault.location.lat, highlightedFault.location.lon]}
              radius={25}
              pathOptions={{
                color: "#ef4444",
                weight: 3,
                fillColor: "#ef4444",
                fillOpacity: 0.1,
                dashArray: "10, 5",
              }}
            />
          )}
        </MapContainer>

        {/* Map Controls */}
        <div className="absolute top-4 right-4 flex flex-col gap-2">
          <div className="flex gap-1 bg-white/90 backdrop-blur rounded-lg shadow-lg p-2 border border-border">
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="p-2 hover:bg-accent rounded transition-colors"
              title={sidebarOpen ? "Hide Sidebar" : "Show Sidebar"}
            >
              {sidebarOpen ? <CaretLeft className="size-5" /> : <CaretRight className="size-5" />}
            </button>
            <button
              onClick={() => getMap()?.flyTo(BANGALORE_CENTER, 13)}
              className="p-2 hover:bg-accent rounded transition-colors"
              title="Reset View"
            >
              <House className="size-5" />
            </button>
          </div>
          <div className="bg-white/90 backdrop-blur rounded-lg shadow-lg p-2 border border-border">
            <label className="flex items-center gap-2 px-2 py-1 cursor-pointer">
              <input type="checkbox" checked={showFaults} onChange={e => setShowFaults(e.target.checked)} className="rounded" />
              <span className="text-sm">Active Faults</span>
            </label>
            <label className="flex items-center gap-2 px-2 py-1 cursor-pointer">
              <input type="checkbox" checked={showKnown} onChange={e => setShowKnown(e.target.checked)} className="rounded" />
              <span className="text-sm">Known Topology</span>
            </label>
            <label className="flex items-center gap-2 px-2 py-1 cursor-pointer">
              <input type="checkbox" checked={showInferred} onChange={e => setShowInferred(e.target.checked)} className="rounded" />
              <span className="text-sm">Inferred Topology</span>
            </label>
            <label className="flex items-center gap-2 px-2 py-1 cursor-pointer">
              <input type="checkbox" checked={showPoles} onChange={e => setShowPoles(e.target.checked)} className="rounded" />
              <span className="text-sm">Poles</span>
            </label>
          </div>
        </div>

        {/* Legend */}
        <div className="absolute bottom-4 left-4 bg-white/90 backdrop-blur rounded-lg shadow-lg p-3 border border-border text-xs">
          <div className="font-medium mb-2">Legend</div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-4 h-4 bg-blue-500 rounded-[2px] transform rotate-45 border-2 border-white" />
            <span>Substation</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-4 h-4 bg-amber-500 rounded-full border-2 border-white flex items-center justify-center">
              <span style={{fontSize: '8px', color: 'white'}}>⚡</span>
            </div>
            <span>DT</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-3 h-3 bg-green-500 rounded-full border-2 border-white" />
            <span>Energized Pole</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-3 h-3 bg-red-500 rounded-full border-2 border-white" />
            <span>Dark Pole</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-6 h-0.5 bg-green-500" />
            <span>Known Edge</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-6 h-0.5 bg-amber-500 border-t-[1px] border-dashed border-amber-500" />
            <span>Inferred Edge</span>
          </div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-6 h-0.5 bg-blue-500 border-t-[1px] border-dashed border-blue-500" />
            <span>Feeder</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-6 h-0.5 bg-red-500 border-t-[1px] border-dashed border-red-500" />
            <span>Fault Zone</span>
          </div>
        </div>
      </div>

      {/* Sidebar */}
      {sidebarOpen && (
        <div className="w-72 bg-card border-l border-border flex flex-col overflow-hidden">
          <div className="p-3 border-b border-border flex items-center justify-between">
            <h3 className="font-semibold">Network Assets</h3>
            <button onClick={() => setSidebarOpen(false)} className="p-1 hover:bg-accent rounded">
              <CaretLeft className="size-5" />
            </button>
          </div>
          
          {selectedAsset && (
            <div className="p-3 border-b border-border bg-muted/50">
              <h4 className="font-medium mb-1">Selected</h4>
              <p className="text-sm font-mono">{selectedAsset.id}</p>
              <p className="text-xs text-muted-foreground">
                {selectedAsset.capacity_kva ? `${selectedAsset.capacity_kva} kVA` : 
                 selectedAsset.dt_id ? `DT: ${selectedAsset.dt_id}` :
                 "Substation"}
              </p>
            </div>
          )}

          <div className="flex-1 overflow-y-auto p-3 space-y-2">
            {topology?.substations.map(sub => (
              <div key={sub.id} className="p-2 hover:bg-accent rounded cursor-pointer transition-colors"
                onClick={() => { setSelectedAsset(sub); getMap()?.flyTo([sub.lat, sub.lon], 15); }}>
                <div className="flex items-center gap-2">
                  <div className="w-6 h-6 bg-blue-500 rounded-[2px] transform rotate-45 flex-shrink-0" />
                  <div>
                    <p className="font-mono text-sm">{sub.id}</p>
                    <p className="text-xs text-muted-foreground">Substation</p>
                  </div>
                </div>
              </div>
            ))}

            {topology?.feeders.map(feeder => (
              <div key={feeder.id} className="p-2 hover:bg-accent rounded cursor-pointer transition-colors"
                onClick={() => { setSelectedAsset(feeder); }}>
                <div className="flex items-center gap-2">
                  <Lightning className="size-4 text-blue-500" />
                  <div>
                    <p className="font-mono text-sm">{feeder.id}</p>
                    <p className="text-xs text-muted-foreground">Feeder</p>
                  </div>
                </div>
              </div>
            ))}

            {topology?.transformers.map(dt => (
              <div key={dt.id} className="p-2 hover:bg-accent rounded cursor-pointer transition-colors"
                onClick={() => { setSelectedAsset(dt); getMap()?.flyTo([dt.lat, dt.lon], 16); }}>
                <div className="flex items-center gap-2">
                  <Circle className="size-4 text-amber-500" />
                  <div>
                    <p className="font-mono text-sm">{dt.id}</p>
                    <p className="text-xs text-muted-foreground">{dt.capacity_kva} kVA</p>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Active Faults in Sidebar */}
          {tickets.length > 0 && (
            <div className="p-3 border-t border-border">
              <h4 className="font-medium mb-2">Active Faults</h4>
              <div className="space-y-2 max-h-48 overflow-y-auto">
                {tickets.map(ticket => (
                  <div key={ticket.id} className="p-2 bg-red-50 border border-red-200 rounded cursor-pointer hover:bg-red-100 transition-colors"
                    onClick={() => { 
                      if (ticket.location) getMap()?.flyTo([ticket.location.lat, ticket.location.lon], 16);
                      navigate(`/map?fault=${ticket.id}`);
                    }}>
                    <p className="font-mono text-xs">{ticket.id}</p>
                    <p className="text-xs text-muted-foreground">{ticket.target_id} • {Math.round(ticket.confidence * 100)}%</p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}