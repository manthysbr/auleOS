import { useState, useEffect, useCallback } from "react"
import { Cpu, RefreshCw, Server, Activity, Clock, CheckCircle2, XCircle, AlertCircle, Loader2 } from "lucide-react"
import { cn } from "@/lib/utils"

interface WorkerSpec {
    image: string
    command?: string[]
    env?: Record<string, string>
    resource_cpu?: number
    resource_mem?: number
    tags?: Record<string, string>
}

interface Worker {
    id: string
    spec: WorkerSpec
    status: "UNKNOWN" | "STARTING" | "HEALTHY" | "UNHEALTHY" | "EXITED"
    created_at: string
    updated_at: string
    metadata?: Record<string, string>
}

function StatusIcon({ status }: { status: Worker["status"] }) {
    switch (status) {
        case "HEALTHY":
            return <CheckCircle2 className="w-4 h-4 text-emerald-400" />
        case "STARTING":
            return <Loader2 className="w-4 h-4 text-blue-400 animate-spin" />
        case "UNHEALTHY":
            return <AlertCircle className="w-4 h-4 text-amber-400" />
        case "EXITED":
            return <XCircle className="w-4 h-4 text-red-400" />
        default:
            return <Activity className="w-4 h-4 text-muted-foreground" />
    }
}

function StatusBadge({ status }: { status: Worker["status"] }) {
    const styles: Record<Worker["status"], string> = {
        HEALTHY: "bg-emerald-500/15 text-emerald-400 border-emerald-500/20",
        STARTING: "bg-blue-500/15 text-blue-400 border-blue-500/20",
        UNHEALTHY: "bg-amber-500/15 text-amber-400 border-amber-500/20",
        EXITED: "bg-red-500/15 text-red-400 border-red-500/20",
        UNKNOWN: "bg-muted/30 text-muted-foreground border-border/30",
    }
    return (
        <span className={cn("px-2 py-0.5 rounded-full text-[10px] font-medium border", styles[status])}>
            {status}
        </span>
    )
}

function formatMemory(bytes: number): string {
    if (!bytes) return ""
    if (bytes < 1024) return `${bytes}B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)}KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(0)}MB`
    return `${(bytes / 1024 / 1024 / 1024).toFixed(1)}GB`
}

function timeAgo(dateStr: string): string {
    const d = new Date(dateStr)
    const diff = Date.now() - d.getTime()
    const sec = Math.floor(diff / 1000)
    if (sec < 60) return `${sec}s ago`
    const min = Math.floor(sec / 60)
    if (min < 60) return `${min}m ago`
    const hr = Math.floor(min / 60)
    if (hr < 24) return `${hr}h ago`
    return `${Math.floor(hr / 24)}d ago`
}

export function WorkersView() {
    const [workers, setWorkers] = useState<Worker[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const fetchWorkers = useCallback(async () => {
        try {
            const res = await fetch("/v1/workers")
            if (!res.ok) throw new Error(await res.text())
            const data = await res.json()
            setWorkers(data.workers ?? [])
            setError(null)
        } catch (e: unknown) {
            setError((e as Error).message)
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchWorkers()
        const id = setInterval(fetchWorkers, 8_000)
        return () => clearInterval(id)
    }, [fetchWorkers])

    const byStatus = {
        healthy: workers.filter(w => w.status === "HEALTHY").length,
        starting: workers.filter(w => w.status === "STARTING").length,
        unhealthy: workers.filter(w => w.status === "UNHEALTHY").length,
        exited: workers.filter(w => w.status === "EXITED").length,
    }

    return (
        <div className="h-full flex flex-col bg-background">
            {/* Header */}
            <div className="flex-shrink-0 px-6 py-4 border-b border-border/50">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
                            <Cpu className="w-4 h-4 text-blue-400" />
                        </div>
                        <div>
                            <h1 className="text-sm font-semibold text-foreground">Workers</h1>
                            <p className="text-xs text-muted-foreground">{workers.length} worker{workers.length !== 1 ? "s" : ""} registered</p>
                        </div>
                    </div>
                    <button
                        onClick={fetchWorkers}
                        className="w-8 h-8 rounded-lg flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                        title="Refresh"
                    >
                        <RefreshCw className={cn("w-4 h-4", loading && "animate-spin")} />
                    </button>
                </div>

                {/* Stats strip */}
                {workers.length > 0 && (
                    <div className="flex items-center gap-4 mt-3">
                        {byStatus.healthy > 0 && (
                            <div className="flex items-center gap-1.5">
                                <div className="w-2 h-2 rounded-full bg-emerald-400" />
                                <span className="text-xs text-muted-foreground">{byStatus.healthy} healthy</span>
                            </div>
                        )}
                        {byStatus.starting > 0 && (
                            <div className="flex items-center gap-1.5">
                                <div className="w-2 h-2 rounded-full bg-blue-400" />
                                <span className="text-xs text-muted-foreground">{byStatus.starting} starting</span>
                            </div>
                        )}
                        {byStatus.unhealthy > 0 && (
                            <div className="flex items-center gap-1.5">
                                <div className="w-2 h-2 rounded-full bg-amber-400" />
                                <span className="text-xs text-muted-foreground">{byStatus.unhealthy} unhealthy</span>
                            </div>
                        )}
                        {byStatus.exited > 0 && (
                            <div className="flex items-center gap-1.5">
                                <div className="w-2 h-2 rounded-full bg-red-400" />
                                <span className="text-xs text-muted-foreground">{byStatus.exited} exited</span>
                            </div>
                        )}
                    </div>
                )}
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-3">
                {/* Error */}
                {error && (
                    <div className="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded-lg px-4 py-2">{error}</div>
                )}

                {/* Loading */}
                {loading && !workers.length && (
                    <div className="flex items-center justify-center py-12 text-muted-foreground text-sm">
                        <RefreshCw className="w-4 h-4 animate-spin mr-2" />
                        Loading workers...
                    </div>
                )}

                {/* Empty */}
                {!loading && !workers.length && (
                    <div className="flex flex-col items-center justify-center py-20 gap-3 text-center">
                        <div className="w-12 h-12 rounded-2xl bg-muted/30 flex items-center justify-center">
                            <Server className="w-6 h-6 text-muted-foreground" />
                        </div>
                        <p className="text-sm font-medium text-muted-foreground">No workers registered</p>
                        <p className="text-xs text-muted-foreground/60">Workers are spawned automatically when jobs are submitted</p>
                    </div>
                )}

                {/* Worker cards */}
                {workers.map(worker => (
                    <div key={worker.id} className="rounded-xl border border-border/50 bg-card/30 hover:bg-card/60 transition-all p-4">
                        <div className="flex items-start gap-3">
                            <div className="mt-0.5">
                                <StatusIcon status={worker.status} />
                            </div>
                            <div className="flex-1 min-w-0">
                                {/* ID + status */}
                                <div className="flex items-center gap-2 flex-wrap">
                                    <span className="text-xs font-mono text-foreground truncate">{worker.id.slice(0, 16)}…</span>
                                    <StatusBadge status={worker.status} />
                                </div>

                                {/* Image */}
                                <p className="text-[11px] text-muted-foreground mt-0.5 truncate">
                                    <span className="text-muted-foreground/50">image:</span> {worker.spec.image || "—"}
                                </p>

                                {/* Resources + tags */}
                                <div className="flex flex-wrap gap-2 mt-1.5">
                                    {(worker.spec.resource_cpu ?? 0) > 0 && (
                                        <span className="flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bg-muted/20 text-muted-foreground">
                                            <Cpu className="w-2.5 h-2.5" />
                                            {worker.spec.resource_cpu}× CPU
                                        </span>
                                    )}
                                    {(worker.spec.resource_mem ?? 0) > 0 && (
                                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted/20 text-muted-foreground">
                                            {formatMemory(worker.spec.resource_mem ?? 0)} RAM
                                        </span>
                                    )}
                                    {Object.entries(worker.spec.tags ?? {}).map(([k, v]) => (
                                        <span key={k} className="text-[10px] px-1.5 py-0.5 rounded bg-primary/10 text-primary/70 border border-primary/10">
                                            {k}={v}
                                        </span>
                                    ))}
                                    {Object.entries(worker.metadata ?? {}).map(([k, v]) => (
                                        <span key={k} className="text-[10px] px-1.5 py-0.5 rounded bg-muted/20 text-muted-foreground">
                                            {k}={v}
                                        </span>
                                    ))}
                                </div>
                            </div>

                            {/* Timestamp */}
                            <div className="flex-shrink-0 flex items-center gap-1 text-[10px] text-muted-foreground/60 mt-0.5">
                                <Clock className="w-2.5 h-2.5" />
                                {timeAgo(worker.created_at)}
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}
