import { Routes, Route } from "react-router-dom"
import { Sidebar } from "@/components/layout/Sidebar"
import { FaultInjectorProvider } from "@/context/FaultInjectorContext"
import { FaultInjector } from "@/pages/FaultInjector"
import { useTopology } from "@/hooks/useTopology"
import { Lightning, Warning } from "@phosphor-icons/react"

export function App() {
  const { data, loading, error } = useTopology()

  return (
    <div className="flex h-svh">
      <Sidebar />
      <main className="flex-1 overflow-hidden">
        {loading && <LoadingState />}
        {error && <ErrorState message={error.message} />}
        {data && (
          <FaultInjectorProvider data={data}>
            <Routes>
              <Route path="/" element={<DashboardPlaceholder />} />
              <Route path="/fault-injector" element={<FaultInjector />} />
            </Routes>
          </FaultInjectorProvider>
        )}
      </main>
    </div>
  )
}

function LoadingState() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
      <Lightning className="size-8 animate-pulse" weight="bold" />
      <p className="text-sm">Loading topology from simulator…</p>
    </div>
  )
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
      <Warning className="size-8 text-destructive" weight="bold" />
      <p className="text-sm font-medium text-foreground">Failed to load topology</p>
      <p className="max-w-md text-xs text-muted-foreground">{message}</p>
      <p className="max-w-md text-xs text-muted-foreground">
        Make sure the simulator is running on port 8081.
      </p>
    </div>
  )
}

function DashboardPlaceholder() {
  return (
    <div className="flex h-full items-center justify-center text-muted-foreground">
      Dashboard — coming soon
    </div>
  )
}

export default App
