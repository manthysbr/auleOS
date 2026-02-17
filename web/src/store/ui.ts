import { create } from "zustand"

type ActiveView = "dashboard" | "project" | "agents" | "tools" | "workflows" | "jobs" | "settings"

interface UIState {
    activeView: ActiveView
    chatPanelOpen: boolean
    commandPaletteOpen: boolean
    activeProjectId: string | null

    // Actions
    setActiveView: (view: ActiveView) => void
    toggleChatPanel: () => void
    setChatPanelOpen: (open: boolean) => void
    toggleCommandPalette: () => void
    setCommandPaletteOpen: (open: boolean) => void
    setActiveProjectId: (id: string | null) => void
    openProject: (id: string) => void
}

export const useUIStore = create<UIState>((set) => ({
    activeView: "dashboard",
    chatPanelOpen: true,
    commandPaletteOpen: false,
    activeProjectId: null,

    setActiveView: (view) => set({ activeView: view }),
    toggleChatPanel: () => set((s) => ({ chatPanelOpen: !s.chatPanelOpen })),
    setChatPanelOpen: (open) => set({ chatPanelOpen: open }),
    toggleCommandPalette: () => set((s) => ({ commandPaletteOpen: !s.commandPaletteOpen })),
    setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),
    setActiveProjectId: (id) => set({ activeProjectId: id }),
    openProject: (id) => set({ activeView: "project", activeProjectId: id }),
}))
