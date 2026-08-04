import { Pause, Play, Lightning } from "@phosphor-icons/react"
import { useSimClock } from "@/hooks/useSimClock"

export function ClockDisplay() {
  const { sim_time, multiplier, paused, pause, resume, error } = useSimClock()

  if (error) {
    return (
      <div className="pointer-events-auto flex items-center gap-2 rounded border border-destructive/50 bg-destructive/10 px-3 py-1.5 text-[10px] text-destructive">
        Clock unavailable
      </div>
    )
  }

  return (
    <div className="pointer-events-auto absolute right-3 top-12 z-10 flex items-center gap-2 rounded border border-border bg-card/90 px-3 py-1.5 backdrop-blur-sm">
      <Lightning className="size-3.5 text-yellow-500" weight="fill" />

      <span className="font-mono text-sm font-medium tabular-nums tracking-tight text-foreground">
        SIM {sim_time}
      </span>

      <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] font-medium text-muted-foreground">
        {multiplier}x
      </span>

      {paused ? (
        <button
          onClick={resume}
          className="flex size-5 items-center justify-center rounded bg-green-500/20 text-green-500 transition-colors hover:bg-green-500/30"
          title="Resume simulation"
        >
          <Play className="size-3" weight="fill" />
        </button>
      ) : (
        <button
          onClick={pause}
          className="flex size-5 items-center justify-center rounded bg-yellow-500/20 text-yellow-500 transition-colors hover:bg-yellow-500/30"
          title="Pause simulation"
        >
          <Pause className="size-3" weight="fill" />
        </button>
      )}
    </div>
  )
}
