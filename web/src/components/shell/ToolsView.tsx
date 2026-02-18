import { useState, useCallback } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Wrench, Puzzle, Cpu, Loader2, Zap, Server, Play, ChevronRight, CheckCircle2, XCircle, FlaskConical, LayoutGrid, Clock } from "lucide-react"
import { cn } from "@/lib/utils"

// ─── Types ────────────────────────────────────────────────────────────────────

interface ToolDTO {
    name: string
    description: string
    execution_type: "native" | "wasm" | "docker" | string
    parameters: {
        type: string
        properties: Record<string, { type: string; description?: string; enum?: string[] }>
        required?: string[]
    }
}

interface RunResult {
    ok: boolean
    tool: string
    result?: unknown
    error?: string
    duration_ms: number
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function execBadge(type: string) {
    if (type === "wasm") return { label: "wasm", cls: "bg-violet-500/10 text-violet-400" }
    if (type === "docker") return { label: "docker", cls: "bg-cyan-500/10 text-cyan-400" }
    return { label: "native", cls: "bg-emerald-500/10 text-emerald-400" }
}

function buildDefaultParams(tool: ToolDTO): string {
    if (!tool.parameters?.properties) return "{}"
    const obj: Record<string, unknown> = {}
    for (const [key, schema] of Object.entries(tool.parameters.properties)) {
        if (schema.enum) obj[key] = schema.enum[0]
        else if (schema.type === "number" || schema.type === "integer") obj[key] = 0
        else if (schema.type === "boolean") obj[key] = false
        else if (schema.type === "array") obj[key] = []
        else obj[key] = ""
    }
    return JSON.stringify({ params: obj }, null, 2)
}

// ─── Playground ───────────────────────────────────────────────────────────────

function ToolPlayground() {
    const [selected, setSelected] = useState<ToolDTO | null>(null)
    const [input, setInput] = useState("")
    const [running, setRunning] = useState(false)
    const [result, setResult] = useState<RunResult | null>(null)
    const [parseError, setParseError] = useState<string | null>(null)

    const { data, isLoading } = useQuery<{ tools: ToolDTO[]; count: number }>({
        queryKey: ["tools-list"],
        queryFn: () => fetch("/v1/tools").then(r => r.json()),
        refetchInterval: 15000,
    })

    const tools = data?.tools ?? []

    const selectTool = useCallback((t: ToolDTO) => {
        setSelected(t)
        setInput(buildDefaultParams(t))
        setResult(null)
        setParseError(null)
    }, [])

    const run = useCallback(async () => {
        if (!selected) return
        let body: unknown
        try {
            body = JSON.parse(input)
            setParseError(null)
        } catch (e: unknown) {
            setParseError((e as Error).message)
            return
        }

        setRunning(true)
        setResult(null)
        try {
            const res = await fetch(`/v1/tools/${encodeURIComponent(selected.name)}/run`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(body),
            })
            const json: RunResult = await res.json()
            setResult(json)
        } catch (e: unknown) {
            setResult({ ok: false, tool: selected.name, error: (e as Error).message, duration_ms: 0 })
        } finally {
            setRunning(false)
        }
    }, [selected, input])

    // Group by execution type
    const groups = tools.reduce<Record<string, ToolDTO[]>>((acc, t) => {
        const k = t.execution_type || "native"
        ;(acc[k] ??= []).push(t)
        return acc
    }, {})
    const groupOrder = ["native", "wasm", "docker"]

    return (
        <div className="flex gap-4 h-full min-h-0">
            {/* Left: Tool list */}
            <div className="w-64 flex-shrink-0 flex flex-col gap-2 overflow-y-auto pr-1">
                {isLoading && (
                    <div className="flex justify-center p-8">
                        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
                    </div>
                )}
                {!isLoading && tools.length === 0 && (
                    <p className="text-xs text-muted-foreground p-4 text-center border border-dashed rounded-xl">
                        No tools registered
                    </p>
                )}
                {groupOrder.map(gk => {
                    const group = groups[gk]
                    if (!group?.length) return null
                    const badge = execBadge(gk)
                    return (
                        <div key={gk}>
                            <div className="flex items-center gap-1.5 px-1 mb-1">
                                <span className={cn("text-[10px] font-semibold px-1.5 py-0.5 rounded-full", badge.cls)}>
                                    {badge.label}
                                </span>
                                <span className="text-[10px] text-muted-foreground">{group.length}</span>
                            </div>
                            {group.map(t => (
                                <button
                                    key={t.name}
                                    onClick={() => selectTool(t)}
                                    className={cn(
                                        "w-full text-left px-3 py-2 rounded-lg text-xs transition-all flex items-center justify-between gap-2 mb-0.5",
                                        selected?.name === t.name
                                            ? "bg-primary/10 text-primary font-medium"
                                            : "hover:bg-accent text-foreground/80"
                                    )}
                                >
                                    <span className="font-mono truncate">{t.name}</span>
                                    <ChevronRight className="w-3 h-3 flex-shrink-0 opacity-50" />
                                </button>
                            ))}
                        </div>
                    )
                })}
            </div>

            {/* Right: Editor + result */}
            <div className="flex-1 flex flex-col gap-3 min-h-0 overflow-y-auto">
                {!selected && (
                    <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-2">
                        <FlaskConical className="w-10 h-10 opacity-20" />
                        <p className="text-sm">Select a tool to test it</p>
                    </div>
                )}
                {selected && (
                    <>
                        {/* Tool header */}
                        <div className="p-4 rounded-xl border bg-card/60 border-border/50">
                            <div className="flex items-start justify-between gap-3">
                                <div>
                                    <p className="text-sm font-semibold font-mono">{selected.name}</p>
                                    <p className="text-xs text-muted-foreground mt-0.5">{selected.description}</p>
                                </div>
                                <span className={cn(
                                    "text-[10px] font-medium px-2 py-0.5 rounded-full flex-shrink-0",
                                    execBadge(selected.execution_type).cls
                                )}>
                                    {execBadge(selected.execution_type).label}
                                </span>
                            </div>
                            {/* Param schema summary */}
                            {selected.parameters?.properties && (
                                <div className="mt-3 flex flex-wrap gap-1">
                                    {Object.entries(selected.parameters.properties).map(([k, v]) => (
                                        <span
                                            key={k}
                                            className={cn(
                                                "text-[10px] font-mono px-2 py-0.5 rounded-full border",
                                                selected.parameters.required?.includes(k)
                                                    ? "border-amber-500/30 text-amber-400 bg-amber-500/5"
                                                    : "border-border/50 text-muted-foreground"
                                            )}
                                        >
                                            {k}: {v.type}
                                            {selected.parameters.required?.includes(k) ? " *" : ""}
                                        </span>
                                    ))}
                                </div>
                            )}
                        </div>

                        {/* JSON input */}
                        <div className="flex flex-col gap-1">
                            <label className="text-xs font-medium text-muted-foreground">
                                Input (JSON)
                            </label>
                            <textarea
                                value={input}
                                onChange={e => { setInput(e.target.value); setParseError(null) }}
                                className={cn(
                                    "w-full h-40 font-mono text-xs p-3 rounded-xl border bg-muted/30 resize-none outline-none focus:ring-1 focus:ring-primary/40 transition-all",
                                    parseError ? "border-red-500/50" : "border-border/50"
                                )}
                                spellCheck={false}
                            />
                            {parseError && (
                                <p className="text-xs text-red-400 pl-1">JSON error: {parseError}</p>
                            )}
                        </div>

                        {/* Run button */}
                        <button
                            onClick={run}
                            disabled={running}
                            className="self-start flex items-center gap-2 px-4 py-2 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:opacity-90 disabled:opacity-50 transition-all"
                        >
                            {running
                                ? <Loader2 className="w-4 h-4 animate-spin" />
                                : <Play className="w-4 h-4" />
                            }
                            {running ? "Running…" : "Run tool"}
                        </button>

                        {/* Result */}
                        {result && (
                            <div className={cn(
                                "rounded-xl border p-4 space-y-2",
                                result.ok
                                    ? "border-emerald-500/30 bg-emerald-500/5"
                                    : "border-red-500/30 bg-red-500/5"
                            )}>
                                <div className="flex items-center gap-2">
                                    {result.ok
                                        ? <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                                        : <XCircle className="w-4 h-4 text-red-400" />
                                    }
                                    <span className="text-xs font-medium">
                                        {result.ok ? "Success" : "Error"}
                                    </span>
                                    <span className="ml-auto flex items-center gap-1 text-[10px] text-muted-foreground">
                                        <Clock className="w-3 h-3" />
                                        {result.duration_ms}ms
                                    </span>
                                </div>
                                <pre className="text-xs font-mono whitespace-pre-wrap break-all text-foreground/80 max-h-60 overflow-y-auto">
                                    {result.error
                                        ? result.error
                                        : JSON.stringify(result.result, null, 2)
                                    }
                                </pre>
                            </div>
                        )}
                    </>
                )}
            </div>
        </div>
    )
}

// ─── Main ─────────────────────────────────────────────────────────────────────

export function ToolsView() {
    const [tab, setTab] = useState<"overview" | "playground">("overview")

    const { data: pluginsData, isLoading: pluginsLoading } = useQuery({
        queryKey: ["plugins"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/plugins")
            if (error) throw error
            return data
        },
        refetchInterval: 10000,
        enabled: tab === "overview",
    })

    const { data: capData, isLoading: capLoading } = useQuery({
        queryKey: ["capabilities"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/capabilities")
            if (error) throw error
            return data
        },
        refetchInterval: 10000,
        enabled: tab === "overview",
    })

    const plugins = pluginsData?.plugins ?? []
    const capabilities = capData?.capabilities ?? []
    const stats = capData?.stats
    const isLoading = pluginsLoading || capLoading

    return (
        <div className="h-full flex flex-col overflow-hidden p-6">
            {/* Header */}
            <div className="flex items-center justify-between mb-5 flex-shrink-0">
                <div className="space-y-0.5">
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

                {/* Tabs */}
                <div className="flex rounded-xl border border-border/50 overflow-hidden bg-muted/30 p-0.5 gap-0.5">
                    {(["overview", "playground"] as const).map(t => (
                        <button
                            key={t}
                            onClick={() => setTab(t)}
                            className={cn(
                                "flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all",
                                tab === t
                                    ? "bg-background text-foreground shadow-sm"
                                    : "text-muted-foreground hover:text-foreground"
                            )}
                        >
                            {t === "overview"
                                ? <LayoutGrid className="w-3 h-3" />
                                : <FlaskConical className="w-3 h-3" />
                            }
                            {t === "overview" ? "Overview" : "Playground"}
                        </button>
                    ))}
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 min-h-0 overflow-y-auto">
                {tab === "overview" && (
                    <div className="space-y-8">
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
                )}

                {tab === "playground" && <ToolPlayground />}
            </div>
        </div>
    )
}
