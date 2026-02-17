import { Bot, Cpu, CheckCircle2, XCircle, Loader2 } from "lucide-react"
import { cn } from "@/lib/utils"
import type { SubAgentEvent } from "@/hooks/useSubAgentStream"

const statusConfig = {
    pending: { icon: Loader2, label: "Aguardando", color: "text-gray-400", bg: "bg-gray-50", border: "border-gray-200" },
    running: { icon: Loader2, label: "Executando", color: "text-blue-500", bg: "bg-blue-50/50", border: "border-blue-200/50" },
    done: { icon: CheckCircle2, label: "Concluído", color: "text-emerald-500", bg: "bg-emerald-50/50", border: "border-emerald-200/50" },
    failed: { icon: XCircle, label: "Falhou", color: "text-red-500", bg: "bg-red-50/50", border: "border-red-200/50" },
}

const personaColorMap: Record<string, string> = {
    blue: "from-blue-500 to-blue-600",
    purple: "from-purple-500 to-purple-600",
    green: "from-emerald-500 to-emerald-600",
    orange: "from-orange-500 to-orange-600",
    red: "from-red-500 to-red-600",
    pink: "from-pink-500 to-pink-600",
    cyan: "from-cyan-500 to-cyan-600",
    yellow: "from-yellow-500 to-yellow-600",
}

interface SubAgentCardProps {
    agent: SubAgentEvent
}

export function SubAgentCard({ agent }: SubAgentCardProps) {
    const status = agent.status ?? "pending"
    const config = statusConfig[status]
    const StatusIcon = config.icon
    const gradient = personaColorMap[agent.persona_color ?? "blue"] ?? personaColorMap.blue

    return (
        <div className={cn(
            "rounded-xl border p-3 space-y-2 transition-all duration-300 animate-fade-in",
            config.bg, config.border
        )}>
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <div className={cn(
                        "w-6 h-6 rounded-full bg-gradient-to-br flex items-center justify-center shadow-sm",
                        gradient
                    )}>
                        <Bot className="w-3 h-3 text-white" />
                    </div>
                    <span className="text-xs font-semibold text-foreground/80">
                        {agent.persona_name || "Sub-Agent"}
                    </span>
                    {agent.model_id && (
                        <span className="text-[10px] font-mono bg-black/5 px-1.5 py-0.5 rounded-md text-foreground/50">
                            {agent.model_id}
                        </span>
                    )}
                </div>
                <div className={cn("flex items-center gap-1", config.color)}>
                    <StatusIcon className={cn("w-3.5 h-3.5", status === "running" && "animate-spin")} />
                    <span className="text-[10px] font-medium">{config.label}</span>
                </div>
            </div>

            {/* Thought */}
            {agent.thought && (
                <div className="text-xs font-mono text-muted-foreground bg-yellow-50/60 border border-yellow-200/40 rounded-lg p-2 flex gap-2 items-start">
                    <Cpu className="w-3 h-3 mt-0.5 text-yellow-600 flex-shrink-0" />
                    <span className="italic">{agent.thought}</span>
                </div>
            )}

            {/* Result */}
            {agent.result && status === "done" && (
                <div className="text-xs text-foreground/80 bg-white/60 border border-black/5 rounded-lg p-2">
                    <p className="whitespace-pre-wrap">{agent.result}</p>
                </div>
            )}

            {/* Error */}
            {agent.error && status === "failed" && (
                <div className="text-xs text-red-700 bg-red-50/60 border border-red-200/40 rounded-lg p-2 font-mono">
                    {agent.error}
                </div>
            )}
        </div>
    )
}

interface SubAgentTreeProps {
    agents: SubAgentEvent[]
}

/**
 * SubAgentTree renders all active sub-agents in a vertical stack within the chat.
 * Appears when the orchestrator spawns sub-agents via the `delegate` tool.
 */
export function SubAgentTree({ agents }: SubAgentTreeProps) {
    if (agents.length === 0) return null

    return (
        <div className="space-y-2 my-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground px-1">
                <div className="w-4 h-px bg-black/10" />
                <span className="font-medium">Sub-Agents ({agents.length})</span>
                <div className="flex-1 h-px bg-black/10" />
            </div>
            {agents.map((agent) => (
                <SubAgentCard key={agent.sub_agent_id} agent={agent} />
            ))}
        </div>
    )
}
