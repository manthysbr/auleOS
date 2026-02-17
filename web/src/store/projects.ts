import { create } from "zustand"
import { api } from "@/lib/api"

export interface Project {
    id: string
    name: string
    description: string
    created_at: string
    updated_at: string
}

export interface Artifact {
    id: string
    project_id?: string
    job_id?: string
    conversation_id?: string
    type: "image" | "text" | "document" | "audio" | "video" | "other"
    name: string
    file_path: string
    mime_type: string
    size_bytes: number
    prompt?: string
    created_at: string
}

interface ProjectState {
    projects: Project[]
    artifacts: Artifact[]
    isLoadingProjects: boolean
    isLoadingArtifacts: boolean

    // Actions
    fetchProjects: () => Promise<void>
    createProject: (name: string, description?: string) => Promise<Project | null>
    deleteProject: (id: string) => Promise<void>
    updateProject: (id: string, name?: string, description?: string) => Promise<void>
    fetchArtifacts: (type?: string) => Promise<void>
    fetchProjectArtifacts: (projectId: string) => Promise<Artifact[]>
    deleteArtifact: (id: string) => Promise<void>
}

export const useProjectStore = create<ProjectState>((set, get) => ({
    projects: [],
    artifacts: [],
    isLoadingProjects: false,
    isLoadingArtifacts: false,

    fetchProjects: async () => {
        set({ isLoadingProjects: true })
        try {
            const { data } = await api.GET("/v1/projects")
            if (data) {
                set({ projects: (data as unknown as Project[]) ?? [] })
            }
        } finally {
            set({ isLoadingProjects: false })
        }
    },

    createProject: async (name, description) => {
        const { data } = await api.POST("/v1/projects", {
            body: { name, description },
        })
        if (data) {
            const proj = data as unknown as Project
            set((s) => ({ projects: [proj, ...s.projects] }))
            return proj
        }
        return null
    },

    deleteProject: async (id) => {
        await api.DELETE("/v1/projects/{id}", { params: { path: { id } } })
        set((s) => ({
            projects: s.projects.filter((p) => p.id !== id),
        }))
    },

    updateProject: async (id, name, description) => {
        const body: Record<string, string> = {}
        if (name !== undefined) body.name = name
        if (description !== undefined) body.description = description
        const { data } = await api.PATCH("/v1/projects/{id}", {
            params: { path: { id } },
            body,
        })
        if (data) {
            const updated = data as unknown as Project
            set((s) => ({
                projects: s.projects.map((p) => (p.id === id ? updated : p)),
            }))
        }
    },

    fetchArtifacts: async (type?: string) => {
        set({ isLoadingArtifacts: true })
        try {
            const { data } = await api.GET("/v1/artifacts", {
                params: { query: type ? { type: type as "image" } : {} },
            })
            if (data) {
                set({ artifacts: (data as unknown as Artifact[]) ?? [] })
            }
        } finally {
            set({ isLoadingArtifacts: false })
        }
    },

    fetchProjectArtifacts: async (projectId) => {
        const { data } = await api.GET("/v1/projects/{id}/artifacts", {
            params: { path: { id: projectId } },
        })
        if (data) {
            return (data as unknown as Artifact[]) ?? []
        }
        return []
    },

    deleteArtifact: async (id) => {
        await api.DELETE("/v1/artifacts/{id}", { params: { path: { id } } })
        const state = get()
        set({ artifacts: state.artifacts.filter((a) => a.id !== id) })
    },
}))
