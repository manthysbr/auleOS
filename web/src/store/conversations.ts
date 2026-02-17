import { create } from "zustand"
import { api } from "@/lib/api"

export interface Conversation {
    id: string
    title: string
    persona_id?: string
    created_at: string
    updated_at: string
}

export interface Message {
    id: string
    conversation_id: string
    role: "user" | "assistant" | "system" | "tool"
    content: string
    thought?: string
    steps?: Array<{
        thought?: string
        action?: string
        action_input?: Record<string, unknown>
        observation?: string
        is_final_answer?: boolean
        final_answer?: string
    }>
    tool_call?: { name: string; args: Record<string, unknown> }
    created_at: string
}

interface ConversationState {
    conversations: Conversation[]
    activeConversationId: string | null
    messages: Message[]
    isLoadingConversations: boolean
    isLoadingMessages: boolean

    // Actions
    fetchConversations: () => Promise<void>
    createConversation: (title?: string) => Promise<Conversation | null>
    selectConversation: (id: string) => Promise<void>
    deleteConversation: (id: string) => Promise<void>
    renameConversation: (id: string, title: string) => Promise<void>
    addLocalMessage: (msg: Message) => void
    clearActive: () => void
}

export const useConversationStore = create<ConversationState>((set, get) => ({
    conversations: [],
    activeConversationId: null,
    messages: [],
    isLoadingConversations: false,
    isLoadingMessages: false,

    fetchConversations: async () => {
        set({ isLoadingConversations: true })
        try {
            const { data } = await api.GET("/v1/conversations")
            if (data) {
                set({
                    conversations: (data as unknown as Conversation[]) ?? [],
                })
            }
        } finally {
            set({ isLoadingConversations: false })
        }
    },

    createConversation: async (title = "New Chat") => {
        const { data } = await api.POST("/v1/conversations", {
            body: { title },
        })
        if (data) {
            const conv = data as unknown as Conversation
            set((s) => ({
                conversations: [conv, ...s.conversations],
                activeConversationId: conv.id,
                messages: [],
            }))
            return conv
        }
        return null
    },

    selectConversation: async (id: string) => {
        set({ activeConversationId: id, isLoadingMessages: true, messages: [] })
        try {
            const { data } = await api.GET("/v1/conversations/{id}/messages", {
                params: { path: { id }, query: { limit: 100 } },
            })
            if (data) {
                set({ messages: (data as unknown as Message[]) ?? [] })
            }
        } finally {
            set({ isLoadingMessages: false })
        }
    },

    deleteConversation: async (id: string) => {
        await api.DELETE("/v1/conversations/{id}", {
            params: { path: { id } },
        })
        const state = get()
        set({
            conversations: state.conversations.filter((c) => c.id !== id),
            ...(state.activeConversationId === id
                ? { activeConversationId: null, messages: [] }
                : {}),
        })
    },

    renameConversation: async (id: string, title: string) => {
        await api.PATCH("/v1/conversations/{id}", {
            params: { path: { id } },
            body: { title },
        })
        set((s) => ({
            conversations: s.conversations.map((c) =>
                c.id === id ? { ...c, title } : c
            ),
        }))
    },

    addLocalMessage: (msg: Message) => {
        set((s) => ({ messages: [...s.messages, msg] }))
    },

    clearActive: () => {
        set({ activeConversationId: null, messages: [] })
    },
}))
