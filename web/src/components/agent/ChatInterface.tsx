import { useState, useRef, useEffect } from "react"
import { Send, Bot, User, Cpu, Terminal } from "lucide-react"
import { cn } from "@/lib/utils"
import { api as client } from "@/lib/api"
import { useConversationStore, type Message } from "@/store/conversations"
import { usePersonaStore } from "@/store/personas"

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
                    model: "llama3.2",
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

                        <div className="space-y-2 min-w-0">
                            {/* Thought Bubble */}
                            {msg.thought && (
                                <div className="text-xs text-muted-foreground bg-yellow-50/50 border border-yellow-200/50 rounded-lg p-2 flex gap-2 items-start animate-fade-in">
                                    <Cpu className="w-3 h-3 mt-0.5 text-yellow-600" />
                                    <span className="font-mono italic">{msg.thought}</span>
                                </div>
                            )}

                            {/* ReAct Steps */}
                            {msg.steps && msg.steps.length > 0 && (
                                <div className="text-xs bg-white border border-black/10 rounded-lg p-3 space-y-2">
                                    {msg.steps.map((step, index) => (
                                        <div key={`${msg.id}-step-${index}`} className="space-y-1 pb-2 last:pb-0 border-b last:border-b-0 border-black/5">
                                            {step.thought && (
                                                <div className="font-mono text-muted-foreground">🧠 Pensamento: {step.thought}</div>
                                            )}
                                            {step.action && (
                                                <div className="font-mono text-foreground/80">
                                                    🛠️ Ação: {step.action}
                                                    {step.action_input && (
                                                        <span> ({JSON.stringify(step.action_input)})</span>
                                                    )}
                                                </div>
                                            )}
                                            {step.observation && (
                                                <div className="font-mono text-foreground/70">👀 Observação: {step.observation}</div>
                                            )}
                                            {step.final_answer && (
                                                <div className="font-mono text-foreground">✅ Resposta: {step.final_answer}</div>
                                            )}
                                        </div>
                                    ))}
                                </div>
                            )}

                            {/* Tool Call Visualization */}
                            {msg.tool_call && (
                                <div className="text-xs font-mono bg-black/80 text-green-400 rounded-lg p-3 border border-green-900/50 shadow-inner">
                                    <div className="flex items-center gap-2 mb-1 text-green-500/80 border-b border-green-900/50 pb-1">
                                        <Terminal className="w-3 h-3" />
                                        <span>Tool: {msg.tool_call.name}</span>
                                    </div>
                                    <pre className="overflow-x-auto whitespace-pre-wrap">
                                        {JSON.stringify(msg.tool_call.args, null, 2)}
                                    </pre>
                                </div>
                            )}

                            {/* Main Content */}
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
