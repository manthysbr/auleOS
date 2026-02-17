import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Wrench, Puzzle, Cpu, Loader2, Zap, Server } from "lucide-react"
import { cn } from "@/lib/utils"

export function ToolsView() {
    const { data: pluginsData, isLoading: pluginsLoading } = useQuery({
        queryKey: ["plugins"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/plugins")
            if (error) throw error
            return data
        },
        refetchInterval: 10000,
    })

    const { data: capData, isLoading: capLoading } = useQuery({
        queryKey: ["capabilities"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/capabilities")
            if (error) throw error
            return data
        },
        refetchInterval: 10000,
    })

    const plugins = pluginsData?.plugins ?? []
    const capabilities = capData?.capabilities ?? []
    const stats = capData?.stats

    const isLoading = pluginsLoading || capLoading

    return (
        <div className="h-full overflow-y-auto p-6 space-y-8">
            {/* Header */}
            <div className="space-y-1">
                <div className="flex items-center gap-2">
                    <Wrench className="w-5 h-5 text-primary" />
                    <h1 className="text-lg font-bold">Tools & Capabilities</h1>
                </div>
                {stats && (
                    <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                            <Zap className="w-3 h-3 text-violet-400" /> {stats.synapse ?? 0} synapse
                        </span>
                        <span className="flex items-center gap-1">
                            <Server className="w-3 h-3 text-blue-400" /> {stats.muscle ?? 0} muscle
                        </span>
                        <span>· {stats.total ?? 0} total</span>
                    </div>
                )}
            </div>

            {isLoading && (
                <div className="flex justify-center p-8">
                    <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
                </div>
            )}

            {/* Synapse Plugins */}
            {!isLoading && (
                <section className="space-y-3">
                    <div className="flex items-center gap-2">
                        <Puzzle className="w-4 h-4 text-violet-400" />
                        <h2 className="text-sm font-semibold">Synapse Plugins</h2>
                        <span className="text-xs text-muted-foreground">({plugins.length})</span>
                    </div>

                    {plugins.length === 0 ? (
                        <div className="text-center p-8 text-muted-foreground text-sm rounded-xl border border-dashed border-border/50">
                            No plugins loaded. Use the chat to create tools with the Forge.
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                            {plugins.map((p) => (
                                <div
                                    key={p.name}
                                    className="p-4 rounded-xl border bg-card/60 border-border/50 hover:border-violet-500/30 transition-all"
                                >
                                    <div className="flex items-center gap-3 mb-2">
                                        <div className="w-9 h-9 rounded-lg flex items-center justify-center bg-violet-500/10">
                                            <Puzzle className="w-4 h-4 text-violet-400" />
                                        </div>
                                        <div className="min-w-0">
                                            <p className="text-sm font-medium font-mono truncate">{p.name}</p>
                                            <p className="text-[10px] text-muted-foreground">v{p.version} · {p.runtime}</p>
                                        </div>
                                    </div>
                                    <p className="text-xs text-muted-foreground line-clamp-2">{p.description}</p>
                                    <div className="mt-3 flex items-center gap-2">
                                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-400">
                                            wasm
                                        </span>
                                        <span className="text-[10px] font-mono text-muted-foreground">
                                            → {p.tool_name}
                                        </span>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            )}

            {/* Capability Map */}
            {!isLoading && (
                <section className="space-y-3">
                    <div className="flex items-center gap-2">
                        <Cpu className="w-4 h-4 text-blue-400" />
                        <h2 className="text-sm font-semibold">Capability Map</h2>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                        {capabilities.map((cap) => {
                            const isSynapse = cap.runtime === "synapse"
                            return (
                                <div
                                    key={cap.capability}
                                    className={cn(
                                        "p-3 rounded-lg border transition-all",
                                        isSynapse
                                            ? "bg-violet-500/5 border-violet-500/20"
                                            : "bg-blue-500/5 border-blue-500/20"
                                    )}
                                >
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs font-mono font-medium">{cap.capability}</span>
                                        <span className={cn(
                                            "text-[10px] font-medium px-1.5 py-0.5 rounded-full",
                                            isSynapse
                                                ? "bg-violet-500/10 text-violet-400"
                                                : "bg-blue-500/10 text-blue-400"
                                        )}>
                                            {cap.runtime}
                                        </span>
                                    </div>
                                    <p className="text-[11px] text-muted-foreground">{cap.description}</p>
                                </div>
                            )
                        })}
                    </div>
                </section>
            )}
        </div>
    )
}
