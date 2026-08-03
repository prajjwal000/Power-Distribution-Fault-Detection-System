import { useContext } from "react"
import { FaultInjectorContext } from "@/context/FaultInjectorContext"

export function useFaultInjector() {
  const ctx = useContext(FaultInjectorContext)
  if (!ctx) {
    throw new Error("useFaultInjector must be used inside FaultInjectorProvider")
  }
  return ctx
}
