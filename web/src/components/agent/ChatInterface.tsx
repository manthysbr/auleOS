import { useState, useRef, useEffect, useCallback } from "react"
import { Send, Bot, User, Cpu, Terminal, ChevronRight, Sparkles, Wrench, ArrowRight } from "lucide-react"
import { cn } from "@/lib/utils"
import { api as client } from "@/lib/api"
import { useConversationStore, type Message } from "@/store/conversations"
import { usePersonaStore } from "@/store/personas"
import { useSubAgentStore } from "@/store/subagents"
import { useSubAgentStream, type SubAgentEvent } from "@/hooks/useSubAgentStream"
import { SubAgentTree } from "./SubAgentCard"

// ── Tool color mapping for distinct visual badges ─────────────────
const toolColorMap: Record<string, { bg: string; text: string; border: string }> = {
    generate_image: { bg: "bg-violet-50", text: "text-violet-700", border: "border-violet-200/60" },
    generate_text: { bg: "bg-sky-50", text: "text-sky-700", border: "border-sky-200/60" },
    delegate: { bg: "bg-amber-50", text: "text-amber-700", border: "border-amber-200/60" },
    list_jobs: { bg: "bg-emerald-50", text: "text-emerald-700", border: "border-emerald-200/60" },
    submit_job: { bg: "bg-rose-50", text: "text-rose-700", border: "border-rose-200/60" },
}

const defaultToolColor = { bg: "bg-gray-50", text: "text-gray-600", border: "border-gray-200/60" }

function getToolColor(name: string) {
    return toolColorMap[name] ?? defaultToolColor
}

function ToolBadge({ name, input }: { name: string; input?: unknown }) {
    const tc = getToolColor(name)
    return (
        <div className={cn(
            "inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 border text-[11px]",
            tc.bg, tc.text, tc.border
        )}>
            <Wrench className="w-3 h-3 flex-shrink-0" />
            <span className="font-mono font-semibold">{name}</span>
            {input != null && (
                <span className="font-mono opacity-60 max-w-[180px] truncate">
                    {typeof input === "string" ? input : JSON.stringify(input)}
                </span>
            )}
        </div>
    )
}

// ── Gemini-style collapsible reasoning ────────────────────────────
function ReasoningDropdown({ thought, steps, toolCall }: {
    thought?: string
    steps?: Message["steps"]
    toolCall?: Message["tool_call"]
}) {
    const [isOpen, setIsOpen] = useState(false)
    const hasToolActions = steps?.some(s => s.action) || toolCall
    const toolNames = [
        ...(steps?.filter(s => s.action).map(s => s.action!) ?? []),
        ...(toolCall?.name ? [toolCall.name] : []),
    ]

    return (
        <div className="text-xs">
            <button
                onClick={() => setIsOpen(v => !v)}
                className="flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors py-1 select-none"
            >
                <Sparkles className={cn(
                    "w-3.5 h-3.5 transition-colors",
                    hasToolActions ? "text-violet-500" : "text-amber-400"
                )} />
                <span className="font-medium">
                    {hasToolActions ? "Usou ferramentas" : "Raciocínio"}
                </span>
                {toolNames.length > 0 && !isOpen && (
                    <span className="text-[10px] font-mono bg-violet-100/80 text-violet-600 px-1.5 py-0.5 rounded-full">
                        {toolNames.join(", ")}
                    </span>
                )}
                <ChevronRight className={cn(
                    "w-3 h-3 transition-transform duration-200",
                    isOpen && "rotate-90"
                )} />
            </button>

            <div className={cn(
                "overflow-hidden transition-all duration-300 ease-out",
                isOpen ? "max-h-[500px] opacity-100 mt-1" : "max-h-0 opacity-0"
            )}>
                <div className="space-y-2 pl-4 border-l-2 border-amber-200/40 pb-1">
                    {thought && (
                        <div className="flex gap-2 items-start">
                            <Cpu className="w-3 h-3 mt-0.5 text-amber-500 flex-shrink-0" />
                            <span className="font-mono italic text-foreground/60">{thought}</span>
                        </div>
                    )}
                    {steps?.map((step, i) => (
                        <div key={i} className="space-y-1.5">
                            {step.thought && (
                                <div className="flex gap-2 items-start">
                                    <Cpu className="w-3 h-3 mt-0.5 text-amber-500 flex-shrink-0" />
                                    <span className="font-mono italic text-foreground/60">{step.thought}</span>
                                </div>
                            )}
                            {step.action && (
                                <ToolBadge name={step.action} input={step.action_input} />
                            )}
                            {step.observation && (
                                <div className="flex gap-2 items-start pl-4">
                                    <ArrowRight className="w-3 h-3 mt-0.5 text-teal-500 flex-shrink-0" />
                                    <span className="font-mono text-foreground/50 text-[11px]">{step.observation}</span>
                                </div>
                            )}
                        </div>
                    ))}
                    {toolCall && (
                        <ToolBadge name={toolCall.name ?? "tool"} input={toolCall.args} />
                    )}
                </div>
            </div>
        </div>
    )
}

interface ChatInterfaceProps {
    onOpenJob?: (jobId: string) => void
}

export function ChatInterface({ onOpenJob }: ChatInterfaceProps) {
    const {
        activeConversationId,
        messages,
        addLocalMessage,
        fetchConversations,
    } = useConversationStore()

    const { activePersonaId } = usePersonaStore()
    const { agents, processEvent, clear: clearSubAgents } = useSubAgentStore()

    // SSE connection for sub-agent events
    const handleSubAgentEvent = useCallback(
        (evt: SubAgentEvent) => processEvent(evt),
        [processEvent]
    )
    useSubAgentStream(activeConversationId, handleSubAgentEvent)

    // Clear sub-agents on conversation switch
    useEffect(() => {
        clearSubAgents()
    }, [activeConversationId, clearSubAgents])

    const [input, setInput] = useState("")
    const [isLoading, setIsLoading] = useState(false)
    const messagesEndRef = useRef<HTMLDivElement>(null)

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
    }

    useEffect(() => {
        scrollToBottom()
    }, [messages])

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!input.trim() || isLoading) return

        const userContent = input.trim()
        setInput("")
        setIsLoading(true)

        // Optimistic local user message
        const userMsg: Message = {
            id: `local-${Date.now()}`,
            conversation_id: activeConversationId ?? "",
            role: "user",
            content: userContent,
            created_at: new Date().toISOString(),
        }
        addLocalMessage(userMsg)

        try {
            const { data, error } = await client.POST("/v1/agent/chat", {
                body: {
                    message: userContent,
                    model: "llama3.2:latest",
                    ...(activeConversationId ? { conversation_id: activeConversationId } : {}),
                    ...(activePersonaId ? { persona_id: activePersonaId } : {}),
                },
            })

            if (error) {
                throw new Error(error.error || "Failed to chat")
            }

            if (data) {
                // If a new conversation was created, update store
                if (data.conversation_id && data.conversation_id !== activeConversationId) {
                    useConversationStore.setState({
                        activeConversationId: data.conversation_id,
                    })
                    // Refresh conversation list
                    fetchConversations()
                }

                const assistantMsg: Message = {
                    id: `local-${Date.now() + 1}`,
                    conversation_id: data.conversation_id ?? activeConversationId ?? "",
                    role: "assistant",
                    content: data.response || "",
                    thought: data.thought ?? undefined,
                    steps: data.steps as Message["steps"],
                    tool_call: data.tool_call as Message["tool_call"],
                    created_at: new Date().toISOString(),
                }
                addLocalMessage(assistantMsg)

                // Check for job ID in tool call
                const toolCall = data.tool_call as { name?: string; args?: Record<string, unknown> } | undefined
                const maybeJobId = toolCall?.args?.job_id
                if (typeof maybeJobId === "string" && onOpenJob) {
                    onOpenJob(maybeJobId)
                }
            }
        } catch (err: unknown) {
            const errorMsg: Message = {
                id: `local-${Date.now() + 1}`,
                conversation_id: activeConversationId ?? "",
                role: "assistant",
                content: `Error: ${err instanceof Error ? err.message : "Unknown error"}`,
                created_at: new Date().toISOString(),
            }
            addLocalMessage(errorMsg)
        } finally {
            setIsLoading(false)
        }
    }

    // Show welcome message when no conversation is active and no messages
    const displayMessages = messages.length > 0
        ? messages
        : [{
            id: "welcome",
            conversation_id: "",
            role: "assistant" as const,
            content: "I am auleOS. How can I help you today?",
            created_at: new Date().toISOString(),
        }]

    return (
        <div className="flex flex-col h-full bg-white/50 backdrop-blur-xl rounded-2xl border border-white/20 shadow-xl overflow-hidden">
            {/* Header */}
            <div className="p-4 border-b border-black/5 flex items-center gap-2 bg-white/40">
                <Bot className="w-5 h-5 text-primary" />
                <h2 className="font-semibold text-foreground/80">Agent Log</h2>
            </div>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto p-4 space-y-6">
                {displayMessages.map((msg) => (
                    <div
                        key={msg.id}
                        className={cn(
                            "flex gap-3 max-w-[85%]",
                            msg.role === "user" ? "ml-auto flex-row-reverse" : ""
                        )}
                    >
                        <div className={cn(
                            "w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 shadow-sm",
                            msg.role === "user" ? "bg-primary text-primary-foreground" : "bg-white border border-black/10 text-foreground"
                        )}>
                            {msg.role === "user" ? <User className="w-4 h-4" /> : <Bot className="w-4 h-4" />}
                        </div>

                        <div className="space-y-1.5 min-w-0">
                            {/* Collapsible Reasoning (Gemini-style) */}
                            {msg.role === "assistant" && (msg.thought || (msg.steps && msg.steps.length > 0) || msg.tool_call) && (
                                <ReasoningDropdown
                                    thought={msg.thought}
                                    steps={msg.steps}
                                    toolCall={msg.tool_call}
                                />
                            )}

                            {/* Main Content Bubble */}
                            <div
                                className={cn(
                                    "p-3 rounded-2xl text-sm shadow-sm",
                                    msg.role === "user"
                                        ? "bg-primary text-primary-foreground rounded-tr-none"
                                        : "bg-white border border-black/5 text-foreground rounded-tl-none"
                                )}
                            >
                                {msg.tool_call?.name === 'generate_image' && msg.tool_call.args?.url ? (
                                    <div className="rounded-lg overflow-hidden border border-black/10 mt-1">
                                        <img src={String(msg.tool_call.args.url)} alt="Generated" className="w-full h-auto object-cover" />
                                    </div>
                                ) : (
                                    <p className="whitespace-pre-wrap">{msg.content}</p>
                                )}
                            </div>
                        </div>
                    </div>
                ))}
                {/* Sub-Agent Activity Tree */}
                {agents.size > 0 && (
                    <SubAgentTree agents={Array.from(agents.values())} />
                )}
                {isLoading && (
                    <div className="flex gap-3 mr-auto">
                        <div className="w-8 h-8 rounded-full bg-white border border-black/10 flex items-center justify-center animate-pulse">
                            <Bot className="w-4 h-4 text-foreground/50" />
                        </div>
                        <div className="bg-white/50 p-3 rounded-2xl rounded-tl-none border border-black/5 flex items-center gap-1">
                            <span className="w-1.5 h-1.5 bg-foreground/40 rounded-full animate-bounce [animation-delay:-0.3s]"></span>
                            <span className="w-1.5 h-1.5 bg-foreground/40 rounded-full animate-bounce [animation-delay:-0.15s]"></span>
                            <span className="w-1.5 h-1.5 bg-foreground/40 rounded-full animate-bounce"></span>
                        </div>
                    </div>
                )}
                <div ref={messagesEndRef} />
            </div>

            {/* Input Area */}
            <form onSubmit={handleSubmit} className="p-4 bg-white/60 border-t border-black/5">
                <div className="flex gap-2">
                    <input
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        placeholder="Ask auleOS to do something..."
                        className="flex-1 bg-white/80 border-0 ring-1 ring-black/10 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-primary/20 focus:outline-none transition-all shadow-sm"
                        disabled={isLoading}
                    />
                    <button
                        type="submit"
                        disabled={isLoading || !input.trim()}
                        className="bg-primary text-primary-foreground rounded-xl w-10 h-10 flex items-center justify-center disabled:opacity-50 hover:bg-primary/90 transition-colors shadow-sm"
                    >
                        <Send className="w-4 h-4" />
                    </button>
                </div>
            </form>
        </div>
    )
}
