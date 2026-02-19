import { Home, FolderKanban, Bot, Wrench, Activity, Settings, Search, Workflow, ScanSearch, CalendarClock, Cpu, BotMessageSquare } from "lucide-react"
import { cn } from "@/lib/utils"
import { useUIStore } from "@/store/ui"
import { useQuery } from "@tanstack/react-query"

const dockItems = [
    { id: "dashboard" as const, icon: Home, label: "Home" },
    { id: "project" as const, icon: FolderKanban, label: "Projects" },
    { id: "agents" as const, icon: Bot, label: "Agents" },
    { id: "workflows" as const, icon: Workflow, label: "Flows" },
    { id: "tools" as const, icon: Wrench, label: "Tools" },
    { id: "jobs" as const, icon: Activity, label: "Jobs" },
    { id: "tasks" as const, icon: CalendarClock, label: "Tasks" },
    { id: "workers" as const, icon: Cpu, label: "Workers" },
    { id: "traces" as const, icon: ScanSearch, label: "Traces" },
    { id: "settings" as const, icon: Settings, label: "Settings" },
] as const

export function Dock() {
    const { activeView, setActiveView, toggleCommandPalette } = useUIStore()

    const { data: inboxStatus } = useQuery({
        queryKey: ["kernel-inbox-badge"],
        queryFn: async () => {
            const res = await fetch("http://localhost:8080/v1/system/inbox")
            if (!res.ok) return { unread_count: 0 }
            return res.json() as Promise<{ unread_count: number }>
        },
        refetchInterval: 10000,
    })

    const kernelUnread = inboxStatus?.unread_count ?? 0

    return (
        <aside className="w-16 flex-shrink-0 flex flex-col items-center py-4 gap-1 bg-background/60 backdrop-blur-xl border-r border-border/50">
            {/* Logo */}
            <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center mb-4 border border-primary/20">
                <span className="text-lg font-bold text-primary">a</span>
            </div>

            {/* Search trigger */}
            <button
                onClick={toggleCommandPalette}
                className="w-10 h-10 rounded-xl flex items-center justify-center text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors mb-2"
                title="Search (⌘K)"
            >
                <Search className="w-4 h-4" />
            </button>

            {/* Nav Items */}
            <nav className="flex flex-col gap-1 flex-1">
                {dockItems.map((item) => {
                    const Icon = item.icon
                    const isActive = activeView === item.id ||
                        (item.id === "project" && activeView === "project")
                    return (
                        <button
                            key={item.id}
                            onClick={() => setActiveView(item.id)}
                            className={cn(
                                "w-10 h-10 rounded-xl flex items-center justify-center transition-all relative group",
                                isActive
                                    ? "bg-primary/10 text-primary shadow-sm"
                                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                            )}
                            title={item.label}
                        >
                            <Icon className="w-[18px] h-[18px]" />
                            {isActive && (
                                <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-primary rounded-r-full" />
                            )}
                            <span className="absolute left-14 px-2 py-1 rounded-md bg-popover text-popover-foreground text-xs whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none shadow-md border border-border/50 z-50">
                                {item.label}
                            </span>
                        </button>
                    )
                })}

                {/* Kernel Inbox — fixed at bottom of nav with unread badge */}
                <button
                    onClick={() => setActiveView("kernel")}
                    className={cn(
                        "w-10 h-10 rounded-xl flex items-center justify-center transition-all relative group mt-auto",
                        activeView === "kernel"
                            ? "bg-violet-500/10 text-violet-600 shadow-sm"
                            : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                    )}
                    title="Kernel Inbox"
                >
                    <BotMessageSquare className="w-[18px] h-[18px]" />
                    {kernelUnread > 0 && (
                        <span className="absolute top-0.5 right-0.5 w-4 h-4 rounded-full bg-violet-500 text-[9px] text-white flex items-center justify-center font-bold leading-none">
                            {kernelUnread > 9 ? "9+" : kernelUnread}
                        </span>
                    )}
                    {activeView === "kernel" && (
                        <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-violet-500 rounded-r-full" />
                    )}
                    <span className="absolute left-14 px-2 py-1 rounded-md bg-popover text-popover-foreground text-xs whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none shadow-md border border-border/50 z-50">
                        Kernel{kernelUnread > 0 ? ` (${kernelUnread})` : ""}
                    </span>
                </button>
            </nav>
        </aside>
    )
}
