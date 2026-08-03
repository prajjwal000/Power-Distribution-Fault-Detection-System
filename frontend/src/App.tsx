import { Routes, Route } from "react-router-dom"
import { Sidebar } from "@/components/layout/Sidebar"
import { FaultInjectorProvider } from "@/context/FaultInjectorContext"
import { FaultInjector } from "@/pages/FaultInjector"
import { mockNetwork } from "@/data/mockTopology"

export function App() {
  return (
    <div className="flex h-svh">
      <Sidebar />
      <main className="flex-1 overflow-hidden">
        <FaultInjectorProvider data={mockNetwork}>
          <Routes>
            <Route path="/" element={<DashboardPlaceholder />} />
            <Route path="/fault-injector" element={<FaultInjector />} />
          </Routes>
        </FaultInjectorProvider>
      </main>
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
