import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Dock } from "@/components/shell/Dock"
import { TopBar } from "@/components/shell/TopBar"
import { BottomBar } from "@/components/shell/BottomBar"
import { ChatPanel } from "@/components/shell/ChatPanel"
import { CommandPalette } from "@/components/shell/CommandPalette"
import { Dashboard } from "@/components/shell/Dashboard"
import { ProjectView } from "@/components/shell/ProjectView"
import { AgentsView } from "@/components/shell/AgentsView"
import { ToolsView } from "@/components/shell/ToolsView"
import { JobsView } from "@/components/shell/JobsView"
import { SettingsPanel } from "@/components/workspace/SettingsPanel"
import { useUIStore } from "@/store/ui"

const queryClient = new QueryClient()

function CenterStage() {
    const { activeView } = useUIStore()

    switch (activeView) {
        case "dashboard":
            return <Dashboard />
        case "project":
            return <ProjectView />
        case "agents":
            return <AgentsView />
        case "tools":
            return <ToolsView />
        case "jobs":
            return <JobsView />
        case "settings":
            return <SettingsPanel onClose={() => useUIStore.getState().setActiveView("dashboard")} />
        default:
            return <Dashboard />
    }
}

export default function DesktopShell() {
    return (
        <QueryClientProvider client={queryClient}>
            <div className="h-screen w-screen flex flex-col bg-background overflow-hidden">
                {/* Top Bar */}
                <TopBar />

                {/* Main area: Dock + Center + Chat */}
                <div className="flex-1 flex min-h-0">
                    {/* Left Dock */}
                    <Dock />

                    {/* Center Stage */}
                    <main className="flex-1 min-w-0 overflow-hidden">
                        <CenterStage />
                    </main>

                    {/* Right Chat Panel */}
                    <ChatPanel />
                </div>

                {/* Bottom Bar */}
                <BottomBar />

                {/* Command Palette (overlay) */}
                <CommandPalette />
            </div>
        </QueryClientProvider>
    )
}
