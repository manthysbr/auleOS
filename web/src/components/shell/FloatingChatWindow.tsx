import { useEffect, useRef, useState, useCallback } from "react"
import {
    MessageSquare, Plus, Trash2, Minus, X, Bot, Search,
    Code, Palette, GripHorizontal, Maximize2,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { useConversationStore } from "@/store/conversations"
import { usePersonaStore, type Persona } from "@/store/personas"
import { useUIStore } from "@/store/ui"
import { ChatInterface } from "@/components/agent/ChatInterface"

const iconMap: Record<string, React.ElementType> = {
    bot: Bot, search: Search, palette: Palette, code: Code,
}
const colorMap: Record<string, { bg: string; text: string; ring: string }> = {
    blue:    { bg: "bg-blue-500/10",    text: "text-blue-500",    ring: "ring-blue-500/40" },
    emerald: { bg: "bg-emerald-500/10", text: "text-emerald-500", ring: "ring-emerald-500/40" },
    violet:  { bg: "bg-violet-500/10",  text: "text-violet-500",  ring: "ring-violet-500/40" },
    amber:   { bg: "bg-amber-500/10",   text: "text-amber-500",   ring: "ring-amber-500/40" },
    cyan:    { bg: "bg-cyan-500/10",    text: "text-cyan-500",    ring: "ring-cyan-500/40" },
    rose:    { bg: "bg-rose-500/10",    text: "text-rose-500",    ring: "ring-rose-500/40" },
}

interface Pos { x: number; y: number }
interface Size { w: number; h: number }

const DEFAULT_SIZE: Size = { w: 400, h: 560 }
const MIN_SIZE: Size = { w: 300, h: 320 }

function PersonaOrb({ persona, isActive, onClick }: { persona: Persona; isActive: boolean; onClick: () => void }) {
    const Icon = iconMap[persona.icon] ?? Bot
    const c = colorMap[persona.color] ?? colorMap.blue
    return (
        <button
            onClick={onClick}
            title={persona.name}
            className={cn(
                "group relative flex items-center rounded-full h-7 overflow-hidden transition-all duration-300",
                "max-w-7 hover:max-w-32",
                isActive
                    ? `${c.bg} ${c.text} ring-2 ring-offset-1 ${c.ring} shadow-sm`
                    : "bg-muted/40 text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
        >
            <div className="w-7 h-7 flex items-center justify-center flex-shrink-0">
                <Icon className="w-3 h-3" />
            </div>
            <span className="pr-2 text-[11px] font-medium whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity delay-75">
                {persona.name}
            </span>
        </button>
    )
}

export function FloatingChatWindow() {
    const { chatWindowOpen, setChatWindowOpen, chatWindowMinimized, setChatWindowMinimized } = useUIStore()
    const { conversations, activeConversationId, fetchConversations, selectConversation, deleteConversation, clearActive } = useConversationStore()
    const { personas, activePersonaId, fetchPersonas, setActivePersona } = usePersonaStore()

    // Position & size state
    const [pos, setPos] = useState<Pos>(() => ({
        x: window.innerWidth - DEFAULT_SIZE.w - 24,
        y: window.innerHeight - DEFAULT_SIZE.h - 48,
    }))
    const [size, setSize] = useState<Size>(DEFAULT_SIZE)
    const [isDragging, setIsDragging] = useState(false)
    const [isResizing, setIsResizing] = useState(false)

    const dragStart = useRef<{ mx: number; my: number; ox: number; oy: number } | null>(null)
    const resizeStart = useRef<{ mx: number; my: number; ow: number; oh: number } | null>(null)
    const windowRef = useRef<HTMLDivElement>(null)

    // --- Drag ---
    const onDragMouseDown = useCallback((e: React.MouseEvent) => {
        e.preventDefault()
        dragStart.current = { mx: e.clientX, my: e.clientY, ox: pos.x, oy: pos.y }
        setIsDragging(true)
    }, [pos])

    // --- Resize ---
    const onResizeMouseDown = useCallback((e: React.MouseEvent) => {
        e.preventDefault()
        e.stopPropagation()
        resizeStart.current = { mx: e.clientX, my: e.clientY, ow: size.w, oh: size.h }
        setIsResizing(true)
    }, [size])

    useEffect(() => {
        if (!isDragging && !isResizing) return

        const onMove = (e: MouseEvent) => {
            if (isDragging && dragStart.current) {
                const dx = e.clientX - dragStart.current.mx
                const dy = e.clientY - dragStart.current.my
                setPos({
                    x: Math.max(0, Math.min(window.innerWidth  - size.w, dragStart.current.ox + dx)),
                    y: Math.max(0, Math.min(window.innerHeight - 40,       dragStart.current.oy + dy)),
                })
            }
            if (isResizing && resizeStart.current) {
                const dw = e.clientX - resizeStart.current.mx
                const dh = e.clientY - resizeStart.current.my
                setSize({
                    w: Math.max(MIN_SIZE.w, resizeStart.current.ow + dw),
                    h: Math.max(MIN_SIZE.h, resizeStart.current.oh + dh),
                })
            }
        }
        const onUp = () => {
            setIsDragging(false)
            setIsResizing(false)
            dragStart.current = null
            resizeStart.current = null
        }
        window.addEventListener("mousemove", onMove)
        window.addEventListener("mouseup", onUp)
        return () => {
            window.removeEventListener("mousemove", onMove)
            window.removeEventListener("mouseup", onUp)
        }
    }, [isDragging, isResizing, size.w])

    useEffect(() => {
        fetchConversations()
        fetchPersonas()
    }, [fetchConversations, fetchPersonas])

    if (!chatWindowOpen) return null

    const minimizedH = 40

    return (
        <div
            ref={windowRef}
            className={cn(
                "fixed z-50 flex flex-col rounded-2xl border border-border/60",
                "bg-background/80 backdrop-blur-xl shadow-2xl shadow-black/20",
                "overflow-hidden transition-[height] duration-200",
                isDragging && "select-none cursor-grabbing",
            )}
            style={{
                left: pos.x,
                top: pos.y,
                width: size.w,
                height: chatWindowMinimized ? minimizedH : size.h,
            }}
        >
            {/* Title bar - drag handle */}
            <div
                onMouseDown={onDragMouseDown}
                className={cn(
                    "flex-shrink-0 h-10 flex items-center justify-between px-3",
                    "border-b border-border/40 select-none",
                    "cursor-grab active:cursor-grabbing",
                )}
            >
                <div className="flex items-center gap-2 text-sm font-medium min-w-0">
                    <GripHorizontal className="w-3.5 h-3.5 text-muted-foreground/50 flex-shrink-0" />
                    <MessageSquare className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
                    <span className="truncate text-xs">Chat</span>
                    {activeConversationId && (
                        <span className="text-[10px] text-muted-foreground/50 truncate font-mono">
                            #{activeConversationId.slice(0, 6)}
                        </span>
                    )}
                </div>
                <div className="flex items-center gap-0.5" onMouseDown={e => e.stopPropagation()}>
                    <WinBtn onClick={clearActive} title="Nova conversa" icon={<Plus className="w-3 h-3" />} />
                    <WinBtn
                        onClick={() => setChatWindowMinimized(!chatWindowMinimized)}
                        title={chatWindowMinimized ? "Expandir" : "Minimizar"}
                        icon={chatWindowMinimized ? <Maximize2 className="w-3 h-3" /> : <Minus className="w-3 h-3" />}
                    />
                    <WinBtn
                        onClick={() => setChatWindowOpen(false)}
                        title="Fechar"
                        icon={<X className="w-3 h-3" />}
                        danger
                    />
                </div>
            </div>

            {!chatWindowMinimized && (
                <>
                    {/* Persona strip */}
                    {personas.length > 0 && (
                        <div className="flex items-center gap-1 px-3 py-1.5 border-b border-border/30 flex-shrink-0 overflow-x-auto scrollbar-none">
                            <PersonaOrb
                                persona={{ id: "__none__", name: "Default", description: "", icon: "bot", color: "blue", system_prompt: "", allowed_tools: [], is_builtin: true, created_at: "", updated_at: "" }}
                                isActive={activePersonaId === null}
                                onClick={() => setActivePersona(null)}
                            />
                            {personas.map(p => (
                                <PersonaOrb
                                    key={p.id}
                                    persona={p}
                                    isActive={activePersonaId === p.id}
                                    onClick={() => setActivePersona(activePersonaId === p.id ? null : p.id)}
                                />
                            ))}
                        </div>
                    )}

                    {/* Conversations list */}
                    <div className="max-h-28 overflow-y-auto border-b border-border/30 flex-shrink-0">
                        {conversations.slice(0, 8).map(conv => (
                            <div
                                key={conv.id}
                                onClick={() => selectConversation(conv.id)}
                                className={cn(
                                    "group flex items-center gap-2 px-3 py-1.5 text-xs cursor-pointer transition-colors",
                                    activeConversationId === conv.id
                                        ? "bg-primary/10 text-primary"
                                        : "hover:bg-accent text-muted-foreground hover:text-foreground"
                                )}
                            >
                                <MessageSquare className="h-3 w-3 flex-shrink-0 opacity-40" />
                                <span className="truncate flex-1">{conv.title || "Untitled"}</span>
                                <button
                                    onClick={e => { e.stopPropagation(); deleteConversation(conv.id) }}
                                    className="opacity-0 group-hover:opacity-100 hover:text-destructive transition-opacity"
                                >
                                    <Trash2 className="h-3 w-3" />
                                </button>
                            </div>
                        ))}
                        {conversations.length === 0 && (
                            <div className="text-center p-2 text-xs text-muted-foreground">Nenhuma conversa</div>
                        )}
                    </div>

                    {/* Chat area */}
                    <div className="flex-1 min-h-0">
                        <ChatInterface />
                    </div>

                    {/* Resize handle */}
                    <div
                        onMouseDown={onResizeMouseDown}
                        className="absolute bottom-0 right-0 w-5 h-5 cursor-se-resize flex items-end justify-end p-1"
                    >
                        <svg width="8" height="8" viewBox="0 0 8 8" className="text-muted-foreground/30">
                            <path d="M7 1L1 7M7 4L4 7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                        </svg>
                    </div>
                </>
            )}
        </div>
    )
}

function WinBtn({ onClick, title, icon, danger }: {
    onClick: () => void; title: string; icon: React.ReactNode; danger?: boolean
}) {
    return (
        <button
            onClick={onClick}
            title={title}
            className={cn(
                "w-6 h-6 rounded-md flex items-center justify-center transition-colors",
                danger
                    ? "text-muted-foreground hover:bg-red-500/20 hover:text-red-400"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            )}
        >
            {icon}
        </button>
    )
}
