import { create } from "zustand"
import { api } from "@/lib/api"

export interface ModelSpec {
    id: string
    name: string
    provider: string
    role: "general" | "code" | "creative" | "fast"
    size: string
    base_url?: string
    is_local: boolean
}

interface ModelState {
    models: ModelSpec[]
    isLoading: boolean
    isDiscovering: boolean

    fetchModels: () => Promise<void>
    discoverModels: () => Promise<void>
}

export const useModelStore = create<ModelState>((set) => ({
    models: [],
    isLoading: false,
    isDiscovering: false,

    fetchModels: async () => {
        set({ isLoading: true })
        try {
            const { data } = await api.GET("/v1/models")
            if (data) {
                const models = (data as unknown as ModelSpec[]) ?? []
                set({ models })
            }
        } finally {
            set({ isLoading: false })
        }
    },

    discoverModels: async () => {
        set({ isDiscovering: true })
        try {
            const { data } = await api.POST("/v1/models/discover")
            if (data) {
                const models = (data as unknown as ModelSpec[]) ?? []
                set({ models })
            }
        } finally {
            set({ isDiscovering: false })
        }
    },
}))
