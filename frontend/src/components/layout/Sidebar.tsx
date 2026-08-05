import { NavLink } from "react-router-dom"
import { GearSix, House, Lightning, MapPin } from "@phosphor-icons/react"

const navItems = [
  {
    group: "Operations",
    items: [
      { to: "/", label: "Dashboard", icon: House },
      { to: "/map", label: "Map", icon: MapPin },
    ],
  },
  {
    group: "Simulation",
    items: [
      { to: "/fault-injector", label: "Fault Injector", icon: Lightning },
    ],
  },
]

export function Sidebar() {
  return (
    <aside className="flex h-full w-52 flex-col border-r border-border bg-sidebar text-sidebar-foreground">
      <div className="flex items-center gap-2 border-b border-border px-4 py-3">
        <GearSix className="size-5 text-sidebar-primary" weight="bold" />
        <span className="text-sm font-semibold tracking-tight">KSPDB</span>
      </div>
      <nav className="flex-1 overflow-y-auto px-2 py-3">
        {navItems.map((group) => (
          <div key={group.group} className="mb-4">
            <div className="mb-1 px-2 text-[10px] font-medium uppercase tracking-widest text-muted-foreground">
              {group.group}
            </div>
            {group.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  `flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors ${
                    isActive
                      ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                      : "text-sidebar-foreground hover:bg-sidebar-accent/50"
                  }`
                }
              >
                <item.icon className="size-4" />
                {item.label}
              </NavLink>
            ))}
          </div>
        ))}
      </nav>
    </aside>
  )
}
