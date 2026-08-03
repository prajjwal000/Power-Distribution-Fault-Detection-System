import { Separator } from "@/components/ui/separator"

export function Legend() {
  return (
    <div className="pointer-events-auto absolute left-3 top-3 rounded-md border border-border bg-card/90 px-3 py-2 text-[10px] backdrop-blur-sm">
      <div className="mb-1 font-medium text-foreground">Legend</div>

      <div className="flex flex-col gap-1">
        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <rect x="1" y="4" width="14" height="8" rx="2" fill="#1e3a5f" stroke="#d4d4d4" strokeWidth="0.8" />
          </svg>
          <span className="text-muted-foreground">Substation</span>
        </div>
        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <rect x="1" y="4" width="14" height="8" rx="2" fill="#2d5016" stroke="#d4d4d4" strokeWidth="0.8" />
          </svg>
          <span className="text-muted-foreground">Feeder</span>
        </div>
        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <rect x="1" y="4" width="14" height="8" rx="2" fill="#5c1a1a" stroke="#d4d4d4" strokeWidth="0.8" />
          </svg>
          <span className="text-muted-foreground">Transformer (DT)</span>
        </div>

        <Separator className="my-0.5" />

        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <circle cx="8" cy="8" r="4" fill="#d4d4d4" stroke="#d4d4d4" strokeWidth="0.8" />
          </svg>
          <span className="text-muted-foreground">Pole (has device)</span>
        </div>
        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <circle cx="8" cy="8" r="4" fill="none" stroke="#737373" strokeWidth="1.2" strokeDasharray="2 2" />
          </svg>
          <span className="text-muted-foreground">Pole (no device)</span>
        </div>

        <Separator className="my-0.5" />

        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <line x1="1" y1="8" x2="15" y2="8" stroke="#555" strokeWidth="1.2" />
          </svg>
          <span className="text-muted-foreground">Known edge</span>
        </div>
        <div className="flex items-center gap-2">
          <svg width="16" height="16">
            <line x1="1" y1="8" x2="15" y2="8" stroke="#666" strokeWidth="1.2" strokeDasharray="3 2" />
          </svg>
          <span className="text-muted-foreground">Unknown edge</span>
        </div>
      </div>
    </div>
  )
}
