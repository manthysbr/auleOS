import { useQuery } from "@tanstack/react-query"
import {
    Activity, ChevronRight, Clock, CheckCircle, XCircle, Loader2,
    Brain, Wrench, Bot, Workflow, Zap, ScanSearch, AlertCircle,
    MessageSquare, Timer,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { useState } from "react"

const API_BASE = "http://localhost:8080"

interface TraceSummary {
    id: string
    name: string
    status: "running" | "ok" | "error" | "cancelled"
    start_time: string
    duration_ms: number
    span_count: number
}

interface Span {
    id: string
    parent_id?: string
    trace_id: string
    name: string
    kind: "agent" | "llm" | "tool" | "sub_agent" | "workflow" | "step"
    status: "running" | "ok" | "error" | "cancelled"
    input?: string
    output?: string
    error?: string
    model?: string
    attributes?: Record<string, string>
    start_time: string
    end_time?: string
    duration_ms?: number
    children?: string[]
}

interface TraceDetail {
    id: string
    root_span_id: string
    name: string
    status: string
    conversation_id?: string
    persona_id?: string
    start_time: string
    end_time?: string
    duration_ms: number
    span_count: number
    spans: Span[]
}

// ---- Tokens per kind ----
const kindMeta: Record<string, { icon: React.ElementType; label: string; bg: string; text: string; bar: string }> = {
    agent:     { icon: Bot,      label: "Agent",     bg: "bg-blue-500/10",   text: "text-blue-400",   bar: "bg-blue-500" },
    llm:       { icon: Brain,    label: "LLM",       bg: "bg-purple-500/10", text: "text-purple-400", bar: "bg-purple-500" },
    tool:      { icon: Wrench,   label: "Tool",      bg: "bg-amber-500/10",  text: "text-amber-400",  bar: "bg-amber-500" },
    sub_agent: { icon: Bot,      label: "Sub-Agent", bg: "bg-cyan-500/10",   text: "text-cyan-400",   bar: "bg-cyan-500" },
    workflow:  { icon: Workflow, label: "Workflow",  bg: "bg-green-500/10",  text: "text-green-400",  bar: "bg-green-500" },
    step:      { icon: Zap,      label: "Step",      bg: "bg-orange-500/10", text: "text-orange-400", bar: "bg-orange-500" },
}

const statusMeta: Record<string, { icon: React.ElementType; text: string }> = {
    running:   { icon: Loader2,       text: "text-blue-400" },
    ok:        { icon: CheckCircle,   text: "text-green-400" },
    error:     { icon: XCircle,       text: "text-red-400" },
    cancelled: { icon: AlertCircle,   text: "text-muted-foreground" },
}

function fmtDuration(ms?: number) {
    if (ms === undefined || ms === null) return "—"
    if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
    return `${ms}ms`
}

// ---- SpanRow ----
function SpanRow({ span, allSpans, depth = 0, traceDuration }: {
    span: Span; allSpans: Span[]; depth?: number; traceDuration: number
}) {
    const [expanded, setExpanded] = useState(depth < 2)
    const [detailOpen, setDetailOpen] = useState(false)
    const km = kindMeta[span.kind] || kindMeta.step
    const sm = statusMeta[span.status] || statusMeta.ok
    const Icon = km.icon
    const StatusIconComp = sm.icon
    const children = allSpans.filter(s => s.parent_id === span.id)

    // Width for timeline bar
    const pct = traceDuration > 0 && span.duration_ms
        ? Math.max(2, Math.round((span.duration_ms / traceDuration) * 100))
        : 0

    return (
        <div>
            <div
                className={cn(
                    "group flex items-start gap-0 border-b border-border/20 hover:bg-accent/20 transition-colors",
                )}
                style={{ paddingLeft: `${depth * 20}px` }}
            >
                {/* Expand toggle */}
                <button
                    onClick={() => setExpanded(!expanded)}
                    className="w-8 h-9 flex items-center justify-center flex-shrink-0 text-muted-foreground/40 hover:text-muted-foreground"
                >
                    {children.length > 0
                        ? <ChevronRight className={cn("w-3.5 h-3.5 transition-transform", expanded && "rotate-90")} />
                        : <span className="w-3.5" />
                    }
                </button>

                {/* Kind badge */}
                <div className={cn("flex items-center justify-center w-6 h-6 rounded-md mt-1.5 flex-shrink-0 mr-2", km.bg)}>
                    <Icon className={cn("w-3.5 h-3.5", km.text)} />
                </div>

                {/* Main content */}
                <button
                    className="flex-1 min-w-0 py-1.5 text-left"
                    onClick={() => setDetailOpen(!detailOpen)}
                >
                    <div className="flex items-center gap-2 mb-0.5">
                        <span className={cn("text-[10px] font-medium px-1.5 py-0.5 rounded", km.bg, km.text)}>
                            {km.label}
                        </span>
                        <span className="text-sm truncate font-mono text-foreground/80">{span.name}</span>
                        {span.model && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-400 font-mono flex-shrink-0">
                                {span.model}
                            </span>
                        )}
                    </div>
                    {/* Timeline bar */}
                    {pct > 0 && (
                        <div className="h-1 rounded-full bg-border/30 overflow-hidden mt-1 max-w-48">
                            <div
                                className={cn("h-full rounded-full", km.bar, span.status === "running" && "animate-pulse")}
                                style={{ width: `${pct}%` }}
                            />
                        </div>
                    )}
                </button>

                {/* Right: status + duration */}
                <div className="flex items-center gap-2 py-1.5 pr-3 flex-shrink-0">
                    <StatusIconComp className={cn("w-3.5 h-3.5", sm.text, span.status === "running" && "animate-spin")} />
                    <span className="text-[11px] text-muted-foreground font-mono w-14 text-right">
                        {fmtDuration(span.duration_ms)}
                    </span>
                </div>
            </div>

            {/* Span detail drawer */}
            {detailOpen && (
                <div
                    className="border-b border-border/20 bg-muted/20 px-8 py-3 text-xs space-y-2"
                    style={{ paddingLeft: `${depth * 20 + 56}px` }}
                >
                    {span.input && (
                        <div>
                            <p className="text-[10px] uppercase font-semibold tracking-wider text-muted-foreground mb-1">Input</p>
                            <pre className="p-2 rounded-lg bg-muted/50 border border-border/50 text-[11px] overflow-x-auto max-h-28 overflow-y-auto whitespace-pre-wrap font-mono text-foreground/70">
                                {span.input}
                            </pre>
                        </div>
                    )}
                    {span.output && (
                        <div>
                            <p className="text-[10px] uppercase font-semibold tracking-wider text-muted-foreground mb-1">Output</p>
                            <pre className="p-2 rounded-lg bg-muted/50 border border-border/50 text-[11px] overflow-x-auto max-h-28 overflow-y-auto whitespace-pre-wrap font-mono text-foreground/70">
                                {span.output}
                            </pre>
                        </div>
                    )}
                    {span.error && (
                        <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-[11px]">
                            <strong>Erro:</strong> {span.error}
                        </div>
                    )}
                    {span.attributes && Object.keys(span.attributes).length > 0 && (
                        <div className="flex flex-wrap gap-2">
                            {Object.entries(span.attributes).map(([k, v]) => (
                                <span key={k} className="text-[10px] px-1.5 py-0.5 rounded bg-muted/60 border border-border/50 font-mono">
                                    {k}: <span className="text-foreground/70">{v}</span>
                                </span>
                            ))}
                        </div>
                    )}
                </div>
            )}

            {/* Children */}
            {expanded && children.map(child => (
                <SpanRow key={child.id} span={child} allSpans={allSpans} depth={depth + 1} traceDuration={traceDuration} />
            ))}
        </div>
    )
}

// ---- TraceDetail ----
function TraceDetail({ traceId }: { traceId: string }) {
    const { data: trace, isLoading } = useQuery<TraceDetail>({
        queryKey: ["trace", traceId],
        queryFn: async () => {
            const res = await fetch(`${API_BASE}/v1/traces/${traceId}`)
            if (!res.ok) throw new Error("Failed to fetch trace")
            return res.json()
        },
        refetchInterval: 3000,
    })

    if (isLoading) return (
        <div className="flex justify-center items-center h-full">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
        </div>
    )
    if (!trace) return null

    const sm = statusMeta[trace.status as string] || statusMeta.ok
    const StatusIconComp = sm.icon
    const rootSpans = trace.spans?.filter(s => !s.parent_id || s.id === trace.root_span_id) || []

    // Count spans by kind
    const byKind = (trace.spans || []).reduce<Record<string, number>>((acc, s) => {
        acc[s.kind] = (acc[s.kind] || 0) + 1
        return acc
    }, {})

    return (
        <div className="flex flex-col h-full">
            {/* Trace header */}
            <div className="flex-shrink-0 px-5 py-4 border-b border-border/50 bg-background/40 backdrop-blur-sm">
                <div className="flex items-start justify-between mb-3">
                    <div className="flex-1 min-w-0 pr-4">
                        <div className="flex items-center gap-2 mb-1">
                            <StatusIconComp className={cn("w-4 h-4 flex-shrink-0", sm.text, trace.status === "running" && "animate-spin")} />
                            <h2 className="text-base font-semibold truncate">{trace.name}</h2>
                        </div>
                        <div className="flex flex-wrap gap-3 text-[11px] text-muted-foreground">
                            <span className="flex items-center gap-1"><Timer className="w-3 h-3" />{fmtDuration(trace.duration_ms)}</span>
                            <span className="flex items-center gap-1"><Zap className="w-3 h-3" />{trace.span_count} spans</span>
                            {trace.conversation_id && (
                                <span className="flex items-center gap-1">
                                    <MessageSquare className="w-3 h-3" />
                                    conv:{trace.conversation_id.slice(0, 8)}
                                </span>
                            )}
                            <span className="flex items-center gap-1">
                                <Clock className="w-3 h-3" />
                                {new Date(trace.start_time).toLocaleString("pt-BR")}
                            </span>
                        </div>
                    </div>
                    <span className={cn(
                        "text-[11px] font-medium px-2.5 py-1 rounded-full border flex-shrink-0",
                        trace.status === "ok"      ? "bg-green-500/10 text-green-400 border-green-500/20" :
                        trace.status === "running" ? "bg-blue-500/10 text-blue-400 border-blue-500/20" :
                        trace.status === "error"   ? "bg-red-500/10 text-red-400 border-red-500/20" :
                        "bg-muted/40 text-muted-foreground border-border/40"
                    )}>
                        {trace.status}
                    </span>
                </div>

                {/* Kind breakdown chips */}
                <div className="flex flex-wrap gap-1.5">
                    {Object.entries(byKind).map(([k, count]) => {
                        const km = kindMeta[k] || kindMeta.step
                        const KIcon = km.icon
                        return (
                            <span key={k} className={cn("flex items-center gap-1 text-[10px] font-medium px-2 py-0.5 rounded-full", km.bg, km.text)}>
                                <KIcon className="w-3 h-3" />
                                {count}× {km.label}
                            </span>
                        )
                    })}
                </div>
            </div>

            {/* Span tree */}
            <div className="flex-1 overflow-y-auto">
                {/* Column headers */}
                <div className="flex items-center gap-0 px-3 py-1 border-b border-border/30 bg-muted/20 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60">
                    <span className="flex-1 pl-16">Span</span>
                    <span className="pr-3 w-28 text-right">Duração</span>
                </div>
                <div className="divide-y divide-border/10">
                    {rootSpans.map(span => (
                        <SpanRow
                            key={span.id}
                            span={span}
                            allSpans={trace.spans || []}
                            traceDuration={trace.duration_ms}
                        />
                    ))}
                </div>
            </div>
        </div>
    )
}

// ---- TracesView ----
export function TracesView() {
    const [selectedTraceId, setSelectedTraceId] = useState<string | null>(null)

    const { data, isLoading } = useQuery<{ traces: TraceSummary[]; count: number }>({
        queryKey: ["traces"],
        queryFn: async () => {
            const res = await fetch(`${API_BASE}/v1/traces?limit=50`)
            if (!res.ok) throw new Error("Failed to fetch traces")
            return res.json()
        },
        refetchInterval: 3000,
    })

    const traces = data?.traces || []
    const running = traces.filter(t => t.status === "running").length

    return (
        <div className="h-full flex flex-col overflow-hidden">
            {/* Top bar */}
            <div className="flex items-center gap-3 px-5 py-3 border-b border-border/50 bg-background/40 backdrop-blur-sm flex-shrink-0">
                <ScanSearch className="w-4 h-4 text-primary" />
                <span className="text-sm font-semibold">Traces</span>
                <span className="text-xs text-muted-foreground">({traces.length} total)</span>
                {running > 0 && (
                    <span className="flex items-center gap-1.5 text-xs text-blue-400 font-medium">
                        <span className="w-2 h-2 rounded-full bg-blue-400 animate-pulse" />
                        {running} em execução
                    </span>
                )}
            </div>

            <div className="flex flex-1 min-h-0">
                {/* Left: trace list */}
                <div className="w-80 flex-shrink-0 border-r border-border/50 flex flex-col overflow-hidden">
                    {isLoading && traces.length === 0 ? (
                        <div className="flex-1 flex items-center justify-center">
                            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
                        </div>
                    ) : traces.length === 0 ? (
                        <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-3 p-6 text-center">
                            <Activity className="w-10 h-10 opacity-20" />
                            <p className="text-sm">Nenhum trace ainda.</p>
                            <p className="text-xs opacity-60">Interaja com o agente para gerar traces.</p>
                        </div>
                    ) : (
                        <div className="flex-1 overflow-y-auto p-2 space-y-1">
                            {traces.map(trace => (
                                <TraceListItem
                                    key={trace.id}
                                    trace={trace}
                                    selected={selectedTraceId === trace.id}
                                    onClick={() => setSelectedTraceId(trace.id)}
                                />
                            ))}
                        </div>
                    )}
                </div>

                {/* Right: detail */}
                <div className="flex-1 overflow-hidden">
                    {selectedTraceId ? (
                        <TraceDetail traceId={selectedTraceId} />
                    ) : (
                        <div className="flex flex-col items-center justify-center h-full text-muted-foreground gap-3">
                            <ScanSearch className="w-12 h-12 opacity-15" />
                            <p className="text-sm">Selecione um trace para inspecionar</p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}

function TraceListItem({ trace, selected, onClick }: {
    trace: TraceSummary; selected: boolean; onClick: () => void
}) {
    const sm = statusMeta[trace.status] || statusMeta.ok
    const StatusIconComp = sm.icon
    const isRunning = trace.status === "running"

    return (
        <button
            onClick={onClick}
            className={cn(
                "w-full text-left rounded-xl px-3 py-2.5 border transition-all",
                selected
                    ? "bg-primary/10 border-primary/30"
                    : isRunning
                        ? "bg-blue-500/5 border-blue-500/15 hover:bg-blue-500/10"
                        : "bg-card/40 border-border/40 hover:bg-accent/30"
            )}
        >
            <div className="flex items-start gap-2">
                <StatusIconComp className={cn("w-3.5 h-3.5 mt-0.5 flex-shrink-0", sm.text, isRunning && "animate-spin")} />
                <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate leading-tight">{trace.name}</p>
                    <div className="flex items-center gap-2 mt-1 text-[10px] text-muted-foreground">
                        <span className="flex items-center gap-0.5"><Zap className="w-3 h-3" />{trace.span_count}</span>
                        <span className="flex items-center gap-0.5"><Timer className="w-3 h-3" />{fmtDuration(trace.duration_ms)}</span>
                        <span className="ml-auto">{new Date(trace.start_time).toLocaleTimeString("pt-BR")}</span>
                    </div>
                </div>
            </div>
        </button>
    )
}
