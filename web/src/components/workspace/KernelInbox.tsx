import { useState, useEffect, useRef, useCallback } from "react"
import { Bot, Send, Loader2, CheckCircle, XCircle, Lightbulb, HelpCircle, Info, X } from "lucide-react"
import { cn } from "@/lib/utils"

const KERNEL_CONV_ID = "conv-kernel-system"
const API_BASE = "http://localhost:8080"

interface KernelMessage {
    id: string
    role: "kernel" | "user" | "assistant"
    content: string
    created_at: string
    metadata?: Record<string, unknown>
}

function getKindIcon(kind?: string) {
    switch (kind) {
        case "job_result": return null   // icon from content emoji
        case "suggestion": return <Lightbulb className="w-3.5 h-3.5 text-amber-500 flex-shrink-0 mt-0.5" />
        case "question":   return <HelpCircle className="w-3.5 h-3.5 text-blue-500 flex-shrink-0 mt-0.5" />
        case "welcome":    return <Bot className="w-3.5 h-3.5 text-violet-500 flex-shrink-0 mt-0.5" />
        default:           return <Info className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0 mt-0.5" />
    }
}

function KernelBubble({ msg }: { msg: KernelMessage }) {
    const kind = msg.metadata?.kind as string | undefined
    const isUser = msg.role === "user"

    if (isUser) {
        return (
            <div className="flex justify-end">
                <div className="max-w-[80%] rounded-2xl rounded-br-sm bg-primary text-primary-foreground px-4 py-2.5 text-sm">
                    {msg.content}
                </div>
            </div>
        )
    }

    return (
        <div className="flex gap-2.5 items-start">
            <div className="w-7 h-7 rounded-full bg-violet-100 border border-violet-200/60 flex items-center justify-center flex-shrink-0 mt-0.5">
                <Bot className="w-3.5 h-3.5 text-violet-600" />
            </div>
            <div className="flex-1 min-w-0">
                <div className="flex items-start gap-1.5 rounded-2xl rounded-bl-sm bg-card/80 border border-border/50 px-4 py-2.5">
                    {getKindIcon(kind)}
                    <p className="text-sm text-foreground/90 whitespace-pre-wrap">{msg.content}</p>
                </div>
                <p className="text-[10px] text-muted-foreground mt-1 ml-1">
                    {new Date(msg.created_at).toLocaleTimeString()}
                </p>
            </div>
        </div>
    )
}

interface KernelInboxProps {
    onClose?: () => void
}

export function KernelInbox({ onClose }: KernelInboxProps) {
    const [messages, setMessages] = useState<KernelMessage[]>([])
    const [loading, setLoading] = useState(true)
    const [input, setInput] = useState("")
    const [sending, setSending] = useState(false)
    const bottomRef = useRef<HTMLDivElement>(null)

    // Load existing messages
    const loadMessages = useCallback(async () => {
        try {
            const res = await fetch(`${API_BASE}/v1/conversations/${KERNEL_CONV_ID}/messages?limit=100`)
            if (res.ok) {
                const data = await res.json()
                if (Array.isArray(data)) {
                    setMessages(data)
                }
            }
        } catch { /* no-op */ }
        setLoading(false)
    }, [])

    useEffect(() => {
        loadMessages()
    }, [loadMessages])

    // SSE for real-time kernel messages
    useEffect(() => {
        const es = new EventSource(`${API_BASE}/v1/conversations/${KERNEL_CONV_ID}/events`)

        es.addEventListener("kernel_message", (e: MessageEvent) => {
            try {
                const data = JSON.parse(e.data)
                const msg: KernelMessage = {
                    id: data.message_id ?? `live-${Date.now()}`,
                    role: "kernel",
                    content: data.content,
                    created_at: new Date().toISOString(),
                    metadata: data.metadata,
                }
                setMessages(prev => {
                    // Avoid duplicates (may arrive via both SSE and loadMessages)
                    if (prev.some(m => m.id === msg.id)) return prev
                    return [...prev, msg]
                })
            } catch { /* no-op */ }
        })

        // Agent replies
        es.addEventListener("agent_message", (e: MessageEvent) => {
            try {
                const data = JSON.parse(e.data)
                if (!data.content) return
                const msg: KernelMessage = {
                    id: data.id ?? `live-${Date.now()}`,
                    role: "assistant",
                    content: data.content,
                    created_at: data.created_at ?? new Date().toISOString(),
                }
                setMessages(prev => {
                    if (prev.some(m => m.id === msg.id)) return prev
                    return [...prev, msg]
                })
            } catch { /* no-op */ }
        })

        return () => es.close()
    }, [])

    // Auto-scroll on new messages
    useEffect(() => {
        bottomRef.current?.scrollIntoView({ behavior: "smooth" })
    }, [messages])

    const handleSend = async () => {
        const text = input.trim()
        if (!text || sending) return

        const userMsg: KernelMessage = {
            id: `local-${Date.now()}`,
            role: "user",
            content: text,
            created_at: new Date().toISOString(),
        }
        setMessages(prev => [...prev, userMsg])
        setInput("")
        setSending(true)

        try {
            await fetch(`${API_BASE}/v1/chat`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ message: text, conversation_id: KERNEL_CONV_ID }),
            })
        } catch { /* no-op */ } finally {
            setSending(false)
        }
    }

    const isEmpty = !loading && messages.length === 0

    return (
        <div className="h-full flex flex-col rounded-2xl overflow-hidden bg-background/95 border border-border/60 shadow-2xl">
            {/* Header */}
            <div className="flex items-center gap-3 px-5 py-4 border-b border-border/50 bg-card/60">
                <div className="w-8 h-8 rounded-full bg-violet-100 border border-violet-200/60 flex items-center justify-center">
                    <Bot className="w-4 h-4 text-violet-600" />
                </div>
                <div className="flex-1">
                    <p className="text-sm font-semibold">Kernel</p>
                    <p className="text-[11px] text-muted-foreground">Sistema · Notificações e sugestões</p>
                </div>
                {onClose && (
                    <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-accent transition-colors">
                        <X className="w-4 h-4 text-muted-foreground" />
                    </button>
                )}
            </div>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {loading && (
                    <div className="flex justify-center p-8">
                        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
                    </div>
                )}

                {isEmpty && (
                    <div className="flex flex-col items-center justify-center h-full gap-3 text-center p-8">
                        <Bot className="w-10 h-10 text-violet-300" />
                        <p className="text-sm text-muted-foreground">
                            O Kernel vai aparecer aqui quando concluir tarefas ou tiver sugestões.
                        </p>
                    </div>
                )}

                {messages.map(msg => (
                    <KernelBubble key={msg.id} msg={msg} />
                ))}

                <div ref={bottomRef} />
            </div>

            {/* Input */}
            <div className="border-t border-border/50 p-3">
                <div className="flex gap-2 items-end">
                    <input
                        type="text"
                        value={input}
                        onChange={e => setInput(e.target.value)}
                        onKeyDown={e => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() } }}
                        placeholder="Responda ao kernel…"
                        className="flex-1 min-h-[38px] px-3 py-2 text-sm rounded-xl bg-card border border-border/60 focus:outline-none focus:border-violet-400/60 resize-none"
                    />
                    <button
                        onClick={handleSend}
                        disabled={!input.trim() || sending}
                        className="w-9 h-9 flex items-center justify-center rounded-xl bg-violet-500 text-white disabled:opacity-40 hover:bg-violet-600 transition-colors flex-shrink-0"
                    >
                        {sending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                    </button>
                </div>
            </div>
        </div>
    )
}
