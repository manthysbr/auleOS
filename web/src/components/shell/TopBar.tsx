import { ChevronRight, MessageSquareText, PanelRightClose, PanelRightOpen } from "lucide-react"
import { useUIStore } from "@/store/ui"
import { useProjectStore } from "@/store/projects"

const viewLabels: Record<string, string> = {
    dashboard: "Dashboard",
    project: "Project",
    agents: "Agents",
    tools: "Tools",
    jobs: "Jobs",
    settings: "Settings",
}

export function TopBar() {
    const { activeView, activeProjectId, chatPanelOpen, toggleChatPanel } = useUIStore()
    const { projects } = useProjectStore()

    const activeProject = projects.find((p) => p.id === activeProjectId)

    return (
        <header className="h-12 flex items-center justify-between px-4 bg-background/60 backdrop-blur-xl border-b border-border/50">
            {/* Left: breadcrumb */}
            <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <span className="font-semibold text-foreground">auleOS</span>
                <ChevronRight className="w-3.5 h-3.5" />
                <span>{viewLabels[activeView] || activeView}</span>
                {activeView === "project" && activeProject && (
                    <>
                        <ChevronRight className="w-3.5 h-3.5" />
                        <span className="text-foreground">{activeProject.name}</span>
                    </>
                )}
            </div>

            {/* Right: chat toggle */}
            <div className="flex items-center gap-2">
                <button
                    onClick={toggleChatPanel}
                    className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors px-2 py-1.5 rounded-lg hover:bg-accent"
                    title={chatPanelOpen ? "Hide Chat (⌘J)" : "Show Chat (⌘J)"}
                >
                    <MessageSquareText className="w-3.5 h-3.5" />
                    <span className="hidden sm:inline">Chat</span>
                    {chatPanelOpen ? (
                        <PanelRightClose className="w-3.5 h-3.5" />
                    ) : (
                        <PanelRightOpen className="w-3.5 h-3.5" />
                    )}
                </button>
            </div>
        </header>
    )
}
