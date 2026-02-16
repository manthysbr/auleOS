import { Sidebar } from "@/components/workspace/Sidebar"
import { AgentStream } from "@/components/workspace/AgentStream"
import { ChatInterface } from "@/components/agent/ChatInterface"
import { SettingsPanel } from "@/components/workspace/SettingsPanel"
import { useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

// Create a client
const queryClient = new QueryClient()

export default function Workspace() {
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [viewMode, setViewMode] = useState<"chat" | "job" | "settings">("chat")

    const handleSelectJob = (id: string) => {
        setSelectedJobId(id)
        setViewMode("job")
    }

    const handleOpenSettings = () => {
        setViewMode("settings")
    }

    return (
        <QueryClientProvider client={queryClient}>
            <div className="h-screen w-full bg-background dot-pattern p-4 flex gap-4 overflow-hidden">
                {/* Left Context Panel (25% width) */}
                <aside className="w-80 flex-shrink-0 hidden md:block">
                    <Sidebar
                        currentJobId={selectedJobId}
                        onSelectJob={handleSelectJob}
                        onOpenSettings={handleOpenSettings}
                    />
                </aside>

                {/* Right Agent Panel (Flex Grow) */}
                <main className="flex-1 min-w-0 flex flex-col gap-4">
                    {/* Top Bar / Navigation could go here */}

                    {viewMode === "settings" ? (
                        <SettingsPanel onClose={() => setViewMode(selectedJobId ? "job" : "chat")} />
                    ) : selectedJobId && viewMode === "job" ? (
                        <div className="h-full relative">
                            <button
                                onClick={() => {
                                    setSelectedJobId(null)
                                    setViewMode("chat")
                                }}
                                className="absolute top-4 right-4 z-10 bg-white/80 p-2 rounded-full border shadow-sm hover:bg-white transition"
                            >
                                ✕
                            </button>
                            <AgentStream jobId={selectedJobId} />
                        </div>
                    ) : (
                        <ChatInterface onOpenJob={handleSelectJob} />
                    )}
                </main>
            </div>
        </QueryClientProvider>
    )
}
