import { useEffect } from "react"
import {
    MessageSquare,
    Plus,
    Trash2,
    X,
    Loader2,
    Bot,
    Search,
    Code,
    Palette,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { useConversationStore } from "@/store/conversations"
import { usePersonaStore, type Persona } from "@/store/personas"
import { useUIStore } from "@/store/ui"
import { ChatInterface } from "@/components/agent/ChatInterface"

// Map persona icon names to Lucide components
const iconMap: Record<string, React.ElementType> = {
    bot: Bot,
    search: Search,
    palette: Palette,
    code: Code,
}

// Map persona color tokens to Tailwind classes
const colorMap: Record<string, { bg: string; text: string; ring: string }> = {
    blue: { bg: "bg-blue-500/10", text: "text-blue-600", ring: "ring-blue-500/40" },
    emerald: { bg: "bg-emerald-500/10", text: "text-emerald-600", ring: "ring-emerald-500/40" },
    violet: { bg: "bg-violet-500/10", text: "text-violet-600", ring: "ring-violet-500/40" },
    amber: { bg: "bg-amber-500/10", text: "text-amber-600", ring: "ring-amber-500/40" },
    cyan: { bg: "bg-cyan-500/10", text: "text-cyan-600", ring: "ring-cyan-500/40" },
    rose: { bg: "bg-rose-500/10", text: "text-rose-600", ring: "ring-rose-500/40" },
}

function PersonaOrb({ persona, isActive, onClick }: { persona: Persona; isActive: boolean; onClick: () => void }) {
    const Icon = iconMap[persona.icon] ?? Bot
    const colors = colorMap[persona.color] ?? colorMap.blue

    return (
        <button
            onClick={onClick}
            title={persona.name}
            className={cn(
                "group relative flex items-center rounded-full h-8 overflow-hidden transition-all duration-300 ease-out",
                "max-w-8 hover:max-w-36",
                isActive
                    ? `${colors.bg} ${colors.text} ring-2 ring-offset-1 ${colors.ring} shadow-sm`
                    : "bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
        >
            <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
                <Icon className={cn("w-3.5 h-3.5 transition-transform duration-200", "group-hover:scale-110")} />
            </div>
            <span className="pr-2.5 text-xs font-medium whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity duration-200 delay-100">
                {persona.name}
            </span>
        </button>
    )
}

export function ChatPanel() {
    const { chatPanelOpen, setChatPanelOpen } = useUIStore()
    const {
        conversations,
        activeConversationId,
        isLoadingConversations,
        fetchConversations,
        selectConversation,
        deleteConversation,
        clearActive,
    } = useConversationStore()

    const {
        personas,
        activePersonaId,
        fetchPersonas,
        setActivePersona,
    } = usePersonaStore()

    useEffect(() => {
        fetchConversations()
        fetchPersonas()
    }, [fetchConversations, fetchPersonas])

    if (!chatPanelOpen) return null

    return (
        <aside className="w-[380px] flex-shrink-0 flex flex-col bg-background/60 backdrop-blur-xl border-l border-border/50 overflow-hidden">
            {/* Header */}
            <div className="h-11 flex items-center justify-between px-3 border-b border-border/50">
                <div className="flex items-center gap-2 text-sm font-medium">
                    <MessageSquare className="w-4 h-4 text-muted-foreground" />
                    <span>Chat</span>
                </div>
                <div className="flex items-center gap-1">
                    <button
                        onClick={clearActive}
                        className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                        title="New Chat"
                    >
                        <Plus className="w-3.5 h-3.5" />
                    </button>
                    <button
                        onClick={() => setChatPanelOpen(false)}
                        className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                        title="Close (⌘J)"
                    >
                        <X className="w-3.5 h-3.5" />
                    </button>
                </div>
            </div>

            {/* Persona Selector */}
            {personas.length > 0 && (
                <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50">
                    <PersonaOrb
                        persona={{
                            id: "__none__",
                            name: "Default",
                            description: "No persona — standard assistant",
                            icon: "bot",
                            color: "blue",
                            system_prompt: "",
                            allowed_tools: [],
                            is_builtin: true,
                            created_at: "",
                            updated_at: "",
                        }}
                        isActive={activePersonaId === null}
                        onClick={() => setActivePersona(null)}
                    />
                    {personas.map((p) => (
                        <PersonaOrb
                            key={p.id}
                            persona={p}
                            isActive={activePersonaId === p.id}
                            onClick={() => setActivePersona(activePersonaId === p.id ? null : p.id)}
                        />
                    ))}
                </div>
            )}

            {/* Conversation list (collapsible) */}
            <div className="max-h-36 overflow-y-auto border-b border-border/50">
                {isLoadingConversations && (
                    <div className="flex justify-center p-3">
                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                    </div>
                )}
                {conversations.slice(0, 10).map((conv) => (
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
                        <MessageSquare className="h-3 w-3 flex-shrink-0 opacity-50" />
                        <span className="truncate flex-1">{conv.title || "Untitled"}</span>
                        {conv.persona_id && (
                            <span className="text-[10px] opacity-50">
                                {personas.find((p) => p.id === conv.persona_id)?.name?.charAt(0) ?? "P"}
                            </span>
                        )}
                        <button
                            onClick={(e) => {
                                e.stopPropagation()
                                deleteConversation(conv.id)
                            }}
                            className="opacity-0 group-hover:opacity-100 hover:text-destructive transition-opacity"
                        >
                            <Trash2 className="h-3 w-3" />
                        </button>
                    </div>
                ))}
                {!isLoadingConversations && conversations.length === 0 && (
                    <div className="text-center p-3 text-xs text-muted-foreground">
                        Start chatting below
                    </div>
                )}
            </div>

            {/* Chat Interface (takes remaining space) */}
            <div className="flex-1 min-h-0">
                <ChatInterface />
            </div>
        </aside>
    )
}
