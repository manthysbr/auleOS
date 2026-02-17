import { useEffect, useCallback, useRef } from "react"

export type SubAgentEvent = {
    sub_agent_id: string
    parent_id: string
    conversation_id: string
    persona_name: string
    persona_color: string
    persona_icon: string
    model_id: string
    status: "pending" | "running" | "done" | "failed"
    thought?: string
    result?: string
    error?: string
}

/**
 * useSubAgentStream — connects to /v1/conversations/{id}/events SSE endpoint
 * and collects sub-agent events in real-time. Returns a Map keyed by sub_agent_id.
 */
export function useSubAgentStream(
    conversationId: string | null,
    onEvent?: (event: SubAgentEvent) => void
) {
    const eventSourceRef = useRef<EventSource | null>(null)

    const connect = useCallback(() => {
        if (!conversationId) return

        // Close existing connection
        if (eventSourceRef.current) {
            eventSourceRef.current.close()
        }

        const es = new EventSource(
            `http://localhost:8080/v1/conversations/${conversationId}/events`
        )
        eventSourceRef.current = es

        es.addEventListener("sub_agent", (event: MessageEvent) => {
            try {
                const data = JSON.parse(event.data) as SubAgentEvent
                onEvent?.(data)
            } catch {
                console.warn("Failed to parse sub_agent event:", event.data)
            }
        })

        es.onerror = () => {
            // SSE auto-reconnects; we just log
            console.debug("SSE connection error, will retry...")
        }
    }, [conversationId, onEvent])

    useEffect(() => {
        connect()
        return () => {
            eventSourceRef.current?.close()
            eventSourceRef.current = null
        }
    }, [connect])
}
