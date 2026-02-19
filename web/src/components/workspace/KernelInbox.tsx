import { useState, useEffect, useCallback, useRef } from "react"
import { Bot, Send, Loader2, CheckCircle2, XCircle, Bell, ChevronDown, ChevronRight, Inbox, MessageSquare, Lightbulb, HelpCircle } from "lucide-react"
import ReactMarkdown from "react-markdown"
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

type MsgKind = "job_completed" | "job_failed" | "suggestion" | "question" | "welcome" | "info" | "user"

function detectKind(msg: KernelMessage): MsgKind {
    if (msg.role === "user") return "user"
    const k = msg.metadata?.kind as string | undefined
    if (k === "job_result") return msg.content.includes("COMPLETED") ? "job_completed" : "job_failed"
    if (k === "suggestion") return "suggestion"
    if (k === "question") return "question"
    if (k === "welcome") return "welcome"
    return "info"
}

function isSystemEmail(msg: KernelMessage): boolean {
    const kind = detectKind(msg)
    return msg.role === "kernel" && (kind === "job_completed" || kind === "job_failed" || kind === "info")
}

function isChatMessage(msg: KernelMessage): boolean {
    return !isSystemEmail(msg)
}

const INBOX_META: Record<string, { icon: React.ReactNode; accent: string }> = {
    job_completed: { icon: <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />, accent: "border-l-emerald-400/50" },
    job_failed:    { icon: <XCircle className="w-3.5 h-3.5 text-red-500" />,          accent: "border-l-red-400/50" },
    info:          { icon: <Bell className="w-3.5 h-3.5 text-muted-foreground/60" />, accent: "border-l-border/60" },
}

function excerpt(text: string, max = 72) {
    const first = text.split("\n").find(l => l.trim()) ?? text
    const clean = first.replace(/[*_`#>]/g, "").trim()
    return clean.length > max ? clean.slice(0, max) + "…" : clean
}

function fmtTime(iso: string) {
    const d = new Date(iso)
    const sameDay = d.toDateString() === new Date().toDateString()
    return sameDay
        ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
        : d.toLocaleDateString([], { day: "2-digit", month: "short" })
}

function MD({ content }: { content: string }) {
    return (
        <ReactMarkdown components={{
            p:          ({ children }) => <p className="mb-1 last:mb-0 leading-relaxed">{children}</p>,
            code:       ({ children }) => <code className="font-mono text-[10px] bg-muted/60 rounded px-1">{children}</code>,
            pre:        ({ children }) => <pre className="font-mono text-[10px] bg-muted/60 rounded p-2 overflow-x-auto my-1">{children}</pre>,
            ul:         ({ children }) => <ul className="list-disc list-inside mb-1 space-y-0.5">{children}</ul>,
            ol:         ({ children }) => <ol className="list-decimal list-inside mb-1 space-y-0.5">{children}</ol>,
            li:         ({ children }) => <li>{children}</li>,
            strong:     ({ children }) => <strong className="font-semibold">{children}</strong>,
            em:         ({ children }) => <em className="italic opacity-80">{children}</em>,
            blockquote: ({ children }) => <blockquote className="border-l-2 border-border pl-2 opacity-70 italic my-1">{children}</blockquote>,
        }}>
            {content}
        </ReactMarkdown>
    )
}

// ─── Inbox tab: accordion rows ────────────────────────────────────────────────

function MailRow({ msg, expanded, onToggle, unread }: {
    msg: KernelMessage; expanded: boolean; onToggle: () => void; unread: boolean
}) {
    const kind = detectKind(msg)
    const meta = INBOX_META[kind] ?? INBOX_META.info

    return (
        <div
            className={cn(
                "group border-l-2 transition-colors cursor-pointer select-none",
                expanded ? "bg-muted/30 border-l-violet-400/70" : `${meta.accent} hover:bg-muted/10`,
            )}
            onClick={onToggle}
        >
            <div className="flex items-center gap-2 px-3 py-1.5">
                <span className="flex-shrink-0 w-4 flex justify-center">{meta.icon}</span>
                <div className="flex-1 min-w-0 flex items-center gap-1.5">
                    {unread && <span className="w-1.5 h-1.5 rounded-full bg-violet-500 flex-shrink-0" />}
                    <span className={cn("text-xs truncate", unread ? "font-medium text-foreground" : "text-muted-foreground")}>
                        {excerpt(msg.content)}
                    </span>
                </div>
                <div className="flex items-center gap-1 flex-shrink-0">
                    <span className="text-[10px] text-muted-foreground/50 tabular-nums">{fmtTime(msg.created_at)}</span>
                    {expanded
                        ? <ChevronDown className="w-3 h-3 text-muted-foreground/40" />
                        : <ChevronRight className="w-3 h-3 text-muted-foreground/20 group-hover:text-muted-foreground/50 transition-colors" />}
                </div>
            </div>
            {expanded && (
                <div className="px-4 pb-2.5 pt-0.5 text-xs text-foreground/75 border-t border-border/20">
                    <MD content={msg.content} />
                </div>
            )}
        </div>
    )
}

// ─── Chat tab: compact message list ───────────────────────────────────────────

const CHAT_ICON: Record<string, React.ReactNode> = {
    suggestion: <Lightbulb className="w-3 h-3 text-amber-400" />,
    question:   <HelpCircle className="w-3 h-3 text-sky-400" />,
    welcome:    <Bot className="w-3 h-3 text-violet-500" />,
}

function ChatLine({ msg }: { msg: KernelMessage }) {
    const kind = detectKind(msg)
    const isUser = kind === "user"

    if (isUser) {
        return (
            <div className="flex justify-end px-3 py-1">
                <span className="max-w-[75%] text-xs bg-primary/10 text-foreground rounded-xl rounded-br-sm px-3 py-1.5 leading-relaxed">
                    {msg.content}
                </span>
            </div>
        )
    }

    return (
        <div className="flex items-start gap-2 px-3 py-1">
            <span className="mt-0.5 flex-shrink-0 w-4 flex justify-center">
                {CHAT_ICON[kind] ?? <Bot className="w-3 h-3 text-violet-400" />}
            </span>
            <div className="flex-1 min-w-0">
                <div className="text-xs text-foreground/85 leading-relaxed">
                    <MD content={msg.content} />
                </div>
                <span className="text-[10px] text-muted-foreground/40 tabular-nums">{fmtTime(msg.created_at)}</span>
            </div>
        </div>
    )
}

// ─── Main component ────────────────────────────────────────────────────────────

export function KernelInbox() {
    const [messages, setMessages] = useState<KernelMessage[]>([])
    const [loading, setLoading] = useState(true)
    const [tab, setTab] = useState<"inbox" | "chat">("chat")
    const [expandedId, setExpandedId] = useState<string | null>(null)
    const [readIds, setReadIds] = useState<Set<string>>(new Set())
    const [input, setInput] = useState("")
    const [sending, setSending] = useState(false)
    const [thinking, setThinking] = useState(false)
    const chatBottomRef = useRef<HTMLDivElement>(null)

    const loadMessages = useCallback(async () => {
        try {
            const res = await fetch(`${API_BASE}/v1/conversations/${KERNEL_CONV_ID}/messages?limit=200`)
            if (res.ok) {
                const data = await res.json()
                if (Array.isArray(data)) setMessages(data)
            }
        } catch { /* no-op */ }
        setLoading(false)
    }, [])

    useEffect(() => { loadMessages() }, [loadMessages])

    useEffect(() => {
        const es = new EventSource(`${API_BASE}/v1/conversations/${KERNEL_CONV_ID}/events`)
        const push = (data: unknown, role: KernelMessage["role"]) => {
            const d = data as Record<string, unknown>
            const msg: KernelMessage = {
                id: (d.message_id ?? d.id ?? `live-${Date.now()}`) as string,
                role,
                content: d.content as string,
                created_at: (d.created_at as string | undefined) ?? new Date().toISOString(),
                metadata: d.metadata as Record<string, unknown> | undefined,
            }
            setMessages(prev => prev.some(m => m.id === msg.id) ? prev : [...prev, msg])
        }
        es.addEventListener("kernel_message", (e: MessageEvent) => { try { push(JSON.parse(e.data), "kernel") } catch { /* no-op */ } })
        es.addEventListener("agent_message",  (e: MessageEvent) => { try { push(JSON.parse(e.data), "assistant") } catch { /* no-op */ } })
        return () => es.close()
    }, [])

    useEffect(() => {
        if (tab === "chat") chatBottomRef.current?.scrollIntoView({ behavior: "smooth" })
    }, [messages, tab])

    const inboxMsgs = messages.filter(isSystemEmail)
    const chatMsgs = messages.filter(isChatMessage)

    const inboxUnread = inboxMsgs.filter(m => !readIds.has(m.id)).length
    const chatUnread  = chatMsgs.filter(m => m.role !== "user" && !readIds.has(m.id)).length

    const markRead = (id: string) => setReadIds(prev => new Set([...prev, id]))

    useEffect(() => {
        if (tab !== "chat") return
        const unreadIncoming = chatMsgs
            .filter(msg => msg.role !== "user" && !readIds.has(msg.id))
            .map(msg => msg.id)
        if (unreadIncoming.length === 0) return
        setReadIds(prev => {
            const next = new Set(prev)
            for (const id of unreadIncoming) next.add(id)
            return next
        })
    }, [tab, chatMsgs, readIds])

    const handleSend = async () => {
        const text = input.trim()
        if (!text || sending) return
        const local: KernelMessage = { id: `local-${Date.now()}`, role: "user", content: text, created_at: new Date().toISOString() }
        setMessages(prev => [...prev, local])
        setInput("")
        setSending(true)
        setThinking(true)
        try {
            const res = await fetch(`${API_BASE}/v1/agent/chat`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ message: text, conversation_id: KERNEL_CONV_ID }),
            })
            if (res.ok) {
                const data = await res.json() as { response?: string; conversation_id?: string }
                if (data.response) {
                    const agentMsg: KernelMessage = {
                        id: `agent-${Date.now()}`,
                        role: "assistant",
                        content: data.response,
                        created_at: new Date().toISOString(),
                        metadata: { kind: "question" },
                    }
                    setMessages(prev => [...prev, agentMsg])
                }
            }
        } catch { /* no-op */ } finally { setSending(false); setThinking(false) }
    }

    return (
        <div className="h-full flex flex-col max-w-lg mx-auto">
            {/* Tab bar */}
            <div className="flex items-center gap-0 border-b border-border/50 px-2 pt-1">
                <button
                    onClick={() => setTab("chat")}
                    className={cn(
                        "flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors",
                        tab === "chat"
                            ? "border-violet-500 text-foreground"
                            : "border-transparent text-muted-foreground hover:text-foreground",
                    )}
                >
                    <MessageSquare className="w-3.5 h-3.5" />
                    Chat
                    {chatUnread > 0 && (
                        <span className="bg-violet-500/15 text-violet-600 text-[9px] font-bold rounded-full px-1.5 py-0.5 leading-none">
                            {chatUnread}
                        </span>
                    )}
                </button>
                <button
                    onClick={() => setTab("inbox")}
                    className={cn(
                        "flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors",
                        tab === "inbox"
                            ? "border-violet-500 text-foreground"
                            : "border-transparent text-muted-foreground hover:text-foreground",
                    )}
                >
                    <Inbox className="w-3.5 h-3.5" />
                    Sistema
                    {inboxUnread > 0 && (
                        <span className="bg-muted text-muted-foreground text-[9px] font-bold rounded-full px-1.5 py-0.5 leading-none">
                            {inboxUnread}
                        </span>
                    )}
                </button>
            </div>

            {/* ── INBOX TAB ── */}
            {tab === "inbox" && (
                <div className="flex-1 overflow-y-auto divide-y divide-border/20">
                    {loading && <div className="flex justify-center p-6"><Loader2 className="w-4 h-4 animate-spin text-muted-foreground" /></div>}
                    {!loading && inboxMsgs.length === 0 && (
                        <div className="flex flex-col items-center gap-2 p-10 text-center">
                            <Inbox className="w-7 h-7 text-muted-foreground/20" />
                            <p className="text-xs text-muted-foreground/60">Nenhum evento de sistema ainda.</p>
                        </div>
                    )}
                    {inboxMsgs.map(msg => (
                        <MailRow
                            key={msg.id}
                            msg={msg}
                            expanded={expandedId === msg.id}
                            onToggle={() => {
                                setExpandedId(prev => prev === msg.id ? null : msg.id)
                                markRead(msg.id)
                            }}
                            unread={!readIds.has(msg.id)}
                        />
                    ))}
                </div>
            )}

            {/* ── CHAT TAB ── */}
            {tab === "chat" && (
                <>
                    <div className="flex-1 overflow-y-auto py-2 space-y-0.5">
                        {loading && <div className="flex justify-center p-6"><Loader2 className="w-4 h-4 animate-spin text-muted-foreground" /></div>}
                        {!loading && chatMsgs.length === 0 && (
                            <div className="flex flex-col items-center gap-2 p-10 text-center">
                                <Bot className="w-7 h-7 text-violet-200" />
                                <p className="text-xs text-muted-foreground/60">O kernel vai falar com você aqui.</p>
                            </div>
                        )}
                        {chatMsgs.map(msg => (
                            <ChatLine key={msg.id} msg={msg} />
                        ))}
                        {thinking && (
                            <div className="flex items-center gap-2 px-3 py-1">
                                <Bot className="w-3 h-3 text-violet-400 flex-shrink-0" />
                                <span className="text-xs text-muted-foreground/60 italic">pensando…</span>
                                <span className="flex gap-0.5">
                                    {[0,1,2].map(i => (
                                        <span key={i} className="w-1 h-1 rounded-full bg-violet-400/60 animate-bounce" style={{ animationDelay: `${i * 150}ms` }} />
                                    ))}
                                </span>
                            </div>
                        )}
                        <div ref={chatBottomRef} />
                    </div>

                    <div className="border-t border-border/50 px-3 py-2 flex gap-2 items-center bg-background/60">
                        <input
                            type="text"
                            value={input}
                            onChange={e => setInput(e.target.value)}
                            onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleSend() } }}
                            placeholder="Responder…"
                            className="flex-1 h-7 px-2.5 text-xs rounded-md bg-muted/50 border border-border/50 focus:outline-none focus:border-violet-400/50 placeholder:text-muted-foreground/40 transition-colors"
                        />
                        <button
                            onClick={handleSend}
                            disabled={!input.trim() || sending}
                            className="w-7 h-7 flex items-center justify-center rounded-md bg-violet-500/90 text-white disabled:opacity-30 hover:bg-violet-600 transition-colors"
                        >
                            {sending ? <Loader2 className="w-3 h-3 animate-spin" /> : <Send className="w-3 h-3" />}
                        </button>
                    </div>
                </>
            )}
        </div>
    )
}
