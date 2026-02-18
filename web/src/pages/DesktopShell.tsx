import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Dock } from "@/components/shell/Dock"
import { TopBar } from "@/components/shell/TopBar"
import { BottomBar } from "@/components/shell/BottomBar"
import { FloatingChatWindow } from "@/components/shell/FloatingChatWindow"
import { CommandPalette } from "@/components/shell/CommandPalette"
import { Dashboard } from "@/components/shell/Dashboard"
import { ProjectView } from "@/components/shell/ProjectView"
import { AgentsView } from "@/components/shell/AgentsView"
import { ToolsView } from "@/components/shell/ToolsView"
import { WorkflowsView } from "@/components/shell/WorkflowsView"
import { JobsView } from "@/components/shell/JobsView"
import { TracesView } from "@/components/shell/TracesView"
import { ScheduledTasksView } from "@/components/shell/ScheduledTasksView"
import { WorkersView } from "@/components/shell/WorkersView"
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
        case "workflows":
            return <WorkflowsView />
        case "tools":
            return <ToolsView />
        case "jobs":
            return <JobsView />
        case "traces":
            return <TracesView />
        case "tasks":
            return <ScheduledTasksView />
        case "workers":
            return <WorkersView />
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
                </div>

                {/* Bottom Bar */}
                <BottomBar />

                {/* Floating Chat Window (overlay) */}
                <FloatingChatWindow />

                {/* Command Palette (overlay) */}
                <CommandPalette />
            </div>
        </QueryClientProvider>
    )
}
