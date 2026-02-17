import { create } from "zustand"
import { api } from "@/lib/api"

export interface Persona {
    id: string
    name: string
    description: string
    system_prompt: string
    icon: string
    color: string
    allowed_tools: string[]
    is_builtin: boolean
    created_at: string
    updated_at: string
}

interface PersonaState {
    personas: Persona[]
    activePersonaId: string | null
    isLoading: boolean

    // Actions
    fetchPersonas: () => Promise<void>
    setActivePersona: (id: string | null) => void
    createPersona: (data: {
        name: string
        system_prompt: string
        description?: string
        icon?: string
        color?: string
        allowed_tools?: string[]
    }) => Promise<Persona | null>
    updatePersona: (
        id: string,
        data: {
            name?: string
            description?: string
            system_prompt?: string
            icon?: string
            color?: string
            allowed_tools?: string[]
        }
    ) => Promise<void>
    deletePersona: (id: string) => Promise<void>
}

export const usePersonaStore = create<PersonaState>((set, get) => ({
    personas: [],
    activePersonaId: null,
    isLoading: false,

    fetchPersonas: async () => {
        set({ isLoading: true })
        try {
            const { data } = await api.GET("/v1/personas")
            if (data) {
                const personas = (data as unknown as Persona[]) ?? []
                set({ personas })
            }
        } finally {
            set({ isLoading: false })
        }
    },

    setActivePersona: (id) => set({ activePersonaId: id }),

    createPersona: async (body) => {
        const { data } = await api.POST("/v1/personas", { body })
        if (data) {
            const persona = data as unknown as Persona
            set((s) => ({ personas: [...s.personas, persona] }))
            return persona
        }
        return null
    },

    updatePersona: async (id, body) => {
        const { data } = await api.PATCH("/v1/personas/{id}", {
            params: { path: { id } },
            body,
        })
        if (data) {
            const updated = data as unknown as Persona
            set((s) => ({
                personas: s.personas.map((p) => (p.id === id ? updated : p)),
            }))
        }
    },

    deletePersona: async (id) => {
        await api.DELETE("/v1/personas/{id}", {
            params: { path: { id } },
        })
        const state = get()
        set({
            personas: state.personas.filter((p) => p.id !== id),
            ...(state.activePersonaId === id ? { activePersonaId: null } : {}),
        })
    },
}))
