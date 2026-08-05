import { useState, useCallback } from "react"
import { useNoise } from "@/hooks/useNoise"
import { Timer, Lightning, Repeat, Warning } from "@phosphor-icons/react"
import type { TelemetryStats } from "@/api/simulator"

const AUTO_RESUME_OPTIONS = [
  { label: "Never", value: 0 },
  { label: "10 sec", value: 10 },
  { label: "30 sec", value: 30 },
  { label: "1 min", value: 60 },
  { label: "5 min", value: 300 },
  { label: "10 min", value: 600 },
] as const

export function NoisePanel() {
  const { loading, error, injectNoise, fetchStats } = useNoise()
  const [autoResumeSecs, setAutoResumeSecs] = useState<number>(60)
  const [deviceDeathCount, setDeviceDeathCount] = useState<number>(1)
  const [duplicateCount, setDuplicateCount] = useState<number>(1)
  const [staleReplayCount, setStaleReplayCount] = useState<number>(1)
  const [stats, setStats] = useState<TelemetryStats | null>(null)
  const [showStats, setShowStats] = useState(false)

  const handleDeviceDeath = useCallback(async () => {
    await injectNoise({ kind: "device_death", count: deviceDeathCount, auto_resume_secs: autoResumeSecs })
  }, [injectNoise, deviceDeathCount, autoResumeSecs])

  const handleDuplicate = useCallback(async () => {
    await injectNoise({ kind: "duplicate", count: duplicateCount })
  }, [injectNoise, duplicateCount])

  const handleStaleReplay = useCallback(async () => {
    await injectNoise({ kind: "stale_replay", count: staleReplayCount })
  }, [injectNoise, staleReplayCount])

  const handleFetchStats = useCallback(async () => {
    const s = await fetchStats()
    if (s) setStats(s)
    setShowStats(true)
  }, [fetchStats])

  return (
    <div className="rounded-md border border-border bg-card p-3">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Lightning className="size-4 text-yellow-500" weight="fill" />
          <div className="text-sm font-medium text-foreground">Noise Controls</div>
        </div>
        <button
          onClick={handleFetchStats}
          disabled={loading}
          className="rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-muted-foreground hover:bg-accent disabled:opacity-50 flex items-center gap-1"
        >
          <Repeat className="size-3" weight="duotone" />
          Refresh Stats
        </button>
      </div>

      {error && (
        <div className="mb-3 rounded border border-destructive bg-destructive/10 p-2 text-[10px] text-destructive">
          {error.message}
        </div>
      )}

      <div className="space-y-3">
        {/* Auto-resume selector */}
        <div>
          <label className="block text-[10px] text-muted-foreground mb-1">Auto-resume after</label>
          <select
            value={autoResumeSecs}
            onChange={(e) => setAutoResumeSecs(Number(e.target.value))}
            disabled={loading}
            className="w-full rounded border border-border bg-card/90 px-2 py-1.5 text-[10px] text-foreground hover:bg-accent disabled:opacity-50"
          >
            {AUTO_RESUME_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        {/* Device Death */}
        <div className="rounded border border-border bg-card/50 p-3">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <Lightning className="size-3.5 text-red-500" weight="fill" />
              <div className="text-xs font-medium text-foreground">Device Death</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-[10px] text-muted-foreground">Count:</label>
            <input
              type="number"
              min="1"
              max="100"
              value={deviceDeathCount}
              onChange={(e) => setDeviceDeathCount(Math.max(1, Math.min(100, Number(e.target.value))))}
              disabled={loading}
              className="w-20 rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-foreground disabled:opacity-50"
            />
            <button
              onClick={handleDeviceDeath}
              disabled={loading}
              className="flex-1 rounded border border-red-500/50 bg-red-500/10 px-2 py-1.5 text-[10px] text-red-500 hover:bg-red-500/20 disabled:opacity-50 flex items-center gap-1"
            >
              <Lightning className="size-3" weight="fill" />
              Kill Telemetry
            </button>
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Stops heartbeats for random energized devices. Power stays on.
          </p>
        </div>

        {/* Duplicate Events */}
        <div className="rounded border border-border bg-card/50 p-3">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <Repeat className="size-3.5 text-blue-500" weight="duotone" />
              <div className="text-xs font-medium text-foreground">Duplicate Events</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-[10px] text-muted-foreground">Count:</label>
            <input
              type="number"
              min="1"
              max="50"
              value={duplicateCount}
              onChange={(e) => setDuplicateCount(Math.max(1, Math.min(50, Number(e.target.value))))}
              disabled={loading}
              className="w-20 rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-foreground disabled:opacity-50"
            />
            <button
              onClick={handleDuplicate}
              disabled={loading}
              className="flex-1 rounded border border-blue-500/50 bg-blue-500/10 px-2 py-1.5 text-[10px] text-blue-500 hover:bg-blue-500/20 disabled:opacity-50 flex items-center gap-1"
            >
              <Repeat className="size-3" weight="duotone" />
              Inject Duplicates
            </button>
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Re-emits recent heartbeat with same sequence number (at-least-once simulation).
          </p>
        </div>

        {/* Stale Replay */}
        <div className="rounded border border-border bg-card/50 p-3">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <Warning className="size-3.5 text-orange-500" weight="fill" />
              <div className="text-xs font-medium text-foreground">Stale Replay</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-[10px] text-muted-foreground">Count:</label>
            <input
              type="number"
              min="1"
              max="50"
              value={staleReplayCount}
              onChange={(e) => setStaleReplayCount(Math.max(1, Math.min(50, Number(e.target.value))))}
              disabled={loading}
              className="w-20 rounded border border-border bg-card/90 px-2 py-1 text-[10px] text-foreground disabled:opacity-50"
            />
            <button
              onClick={handleStaleReplay}
              disabled={loading}
              className="flex-1 rounded border border-orange-500/50 bg-orange-500/10 px-2 py-1.5 text-[10px] text-orange-500 hover:bg-orange-500/20 disabled:opacity-50 flex items-center gap-1"
            >
              <Warning className="size-3" weight="fill" />
              Inject Stale Replays
            </button>
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Emits old power_lost events with past timestamps (6h retry simulation).
          </p>
        </div>

        {/* Stats */}
        {showStats && stats && (
          <div className="rounded border border-border bg-card/50 p-3">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <Timer className="size-3.5 text-green-500" weight="fill" />
                <div className="text-xs font-medium text-foreground">Delivery Stats</div>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2 text-[10px]">
              <div className="rounded border border-border bg-card p-2">
                <div className="text-muted-foreground">Events Sent</div>
                <div className="font-mono text-foreground">{stats.events_delivered} / {stats.events_attempted}</div>
              </div>
              <div className="rounded border border-border bg-card p-2">
                <div className="text-muted-foreground">Power Lost Delivered</div>
                <div className="font-mono text-foreground">{stats.power_lost_delivered} / {stats.power_lost_attempted}</div>
              </div>
              <div className="rounded border border-border bg-card p-2">
                <div className="text-muted-foreground">Device Deaths</div>
                <div className="font-mono text-foreground">{stats.device_deaths}</div>
              </div>
              <div className="rounded border border-border bg-card p-2">
                <div className="text-muted-foreground">Device Resumes</div>
                <div className="font-mono text-foreground">{stats.device_resumes}</div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}