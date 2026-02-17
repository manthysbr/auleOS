import { create } from "zustand"
import type { SubAgentEvent } from "@/hooks/useSubAgentStream"

interface SubAgentState {
    /** Active sub-agents keyed by sub_agent_id */
    agents: Map<string, SubAgentEvent>

    /** Process an incoming SSE event — upsert into the map */
    processEvent: (event: SubAgentEvent) => void

    /** Clear all sub-agents (e.g. on conversation switch) */
    clear: () => void
}

export const useSubAgentStore = create<SubAgentState>((set) => ({
    agents: new Map(),

    processEvent: (event) =>
        set((state) => {
            const next = new Map(state.agents)
            next.set(event.sub_agent_id, event)
            return { agents: next }
        }),

    clear: () => set({ agents: new Map() }),
}))
