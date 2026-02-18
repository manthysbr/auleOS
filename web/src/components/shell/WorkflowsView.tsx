import { useState, useEffect } from "react"
import type { components } from "@/lib/api.schema"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
    Play, Pause, Clock, CheckCircle2, XCircle, Layout, Activity,
    Terminal, ChevronRight, Loader2, Zap, GitBranch, AlertCircle,
    RefreshCw, Eye,
} from "lucide-react"
import { cn } from "@/lib/utils"

type Workflow = components["schemas"]["Workflow"]
type WorkflowStep = components["schemas"]["WorkflowStep"]

export function WorkflowsView() {
    const [workflows, setWorkflows] = useState<Workflow[]>([])
    const [selectedId, setSelectedId] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)

    const fetchWorkflows = async () => {
        setLoading(true)
        try {
            const { data, error } = await api.GET("/v1/workflows")
            if (error) { console.error("Failed to fetch workflows:", error); return }
            if (data) setWorkflows(data)
        } catch (e) {
            console.error("Error fetching workflows:", e)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => { fetchWorkflows() }, [])

    const selectedWorkflow = workflows.find(w => w.id === selectedId)

    const running   = workflows.filter(w => w.status === "running").length
    const pending   = workflows.filter(w => w.status === "pending").length
    const completed = workflows.filter(w => w.status === "completed").length
    const failed    = workflows.filter(w => w.status === "failed").length

    return (
        <div className="h-full flex flex-col overflow-hidden">
            <div className="flex items-center gap-3 px-5 py-3 border-b border-border/50 bg-background/40 backdrop-blur-sm flex-shrink-0">
                <Layout className="w-4 h-4 text-primary" />
                <span className="text-sm font-semibold">Workflows</span>
                <div className="flex-1" />
                <div className="flex items-center gap-2">
                    <Chip color="blue"  label="Running"  value={running} />
                    <Chip color="amber" label="Pending"  value={pending} />
                    <Chip color="green" label="Done"     value={completed} />
                    <Chip color="red"   label="Failed"   value={failed} />
                </div>
                <button
                    onClick={fetchWorkflows}
                    disabled={loading}
                    className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                    <RefreshCw className={cn("w-3.5 h-3.5", loading && "animate-spin")} />
                </button>
            </div>

            <div className="flex flex-1 min-h-0">
                <div className="w-72 flex-shrink-0 border-r border-border/50 flex flex-col overflow-hidden">
                    {loading && workflows.length === 0 ? (
                        <div className="flex-1 flex items-center justify-center">
                            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
                        </div>
                    ) : workflows.length === 0 ? (
                        <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-3 p-6 text-center">
                            <Layout className="w-10 h-10 opacity-20" />
                            <p className="text-sm">Nenhum workflow criado ainda.</p>
                            <p className="text-xs opacity-60">Peça ao agente: "crie um workflow…"</p>
                        </div>
                    ) : (
                        <div className="flex-1 overflow-y-auto p-2 space-y-1">
                            {workflows.map(wf => (
                                <WorkflowCard
                                    key={wf.id}
                                    wf={wf}
                                    selected={selectedId === wf.id}
                                    onClick={() => setSelectedId(wf.id ?? null)}
                                />
                            ))}
                        </div>
                    )}
                </div>

                <div className="flex-1 overflow-hidden">
                    {selectedWorkflow ? (
                        <WorkflowDetail wf={selectedWorkflow} onRefresh={fetchWorkflows} />
                    ) : (
                        <div className="flex flex-col items-center justify-center h-full text-muted-foreground gap-3">
                            <Eye className="w-12 h-12 opacity-15" />
                            <p className="text-sm">Selecione um workflow para inspecionar</p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}

function Chip({ color, label, value }: { color: "blue" | "amber" | "green" | "red"; label: string; value: number }) {
    const colors = {
        blue:  "bg-blue-500/10 text-blue-400",
        amber: "bg-amber-500/10 text-amber-400",
        green: "bg-green-500/10 text-green-400",
        red:   "bg-red-500/10 text-red-400",
    }
    return (
        <div className={cn("flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium", colors[color])}>
            <span className="font-mono tabular-nums">{value}</span>
            <span className="opacity-70">{label}</span>
        </div>
    )
}

function WorkflowCard({ wf, selected, onClick }: { wf: Workflow; selected: boolean; onClick: () => void }) {
    const steps = wf.steps?.length ?? 0
    const doneSteps = wf.steps?.filter(s => s.status === "done" || s.status === "completed").length ?? 0
    const progress = steps > 0 ? Math.round((doneSteps / steps) * 100) : 0

    return (
        <button
            onClick={onClick}
            className={cn(
                "w-full text-left rounded-xl px-3 py-2.5 border transition-all group",
                selected
                    ? "bg-primary/10 border-primary/30 shadow-sm"
                    : "bg-card/40 border-border/40 hover:bg-accent/30 hover:border-border/60"
            )}
        >
            <div className="flex items-center justify-between mb-1">
                <span className="text-sm font-medium truncate pr-2">{wf.name}</span>
                <StatusDot status={wf.status} />
            </div>
            {wf.description && (
                <p className="text-[11px] text-muted-foreground line-clamp-1 mb-1.5">{wf.description}</p>
            )}
            <div className="flex items-center gap-2 mt-1.5">
                <div className="flex-1 h-1 rounded-full bg-border/60 overflow-hidden">
                    <div
                        className={cn(
                            "h-full rounded-full transition-all duration-500",
                            wf.status === "failed"    ? "bg-red-500" :
                            wf.status === "running"   ? "bg-blue-500 animate-pulse" :
                            wf.status === "completed" ? "bg-green-500" :
                            "bg-primary/40"
                        )}
                        style={{ width: `${wf.status === "completed" ? 100 : progress}%` }}
                    />
                </div>
                <span className="text-[10px] text-muted-foreground tabular-nums">{doneSteps}/{steps}</span>
            </div>
            <p className="text-[10px] text-muted-foreground/50 mt-1.5">
                {new Date(wf.created_at || "").toLocaleDateString("pt-BR")}
            </p>
        </button>
    )
}

function WorkflowDetail({ wf, onRefresh }: { wf: Workflow; onRefresh: () => void }) {
    return (
        <div className="flex flex-col h-full overflow-hidden">
            <div className="flex-shrink-0 px-6 py-4 border-b border-border/50 bg-background/40 backdrop-blur-sm">
                <div className="flex items-start justify-between">
                    <div>
                        <div className="flex items-center gap-2 mb-1">
                            <GitBranch className="w-5 h-5 text-primary/70" />
                            <h1 className="text-lg font-bold">{wf.name}</h1>
                            <StatusBadge status={wf.status} />
                        </div>
                        {wf.description && (
                            <p className="text-sm text-muted-foreground max-w-lg">{wf.description}</p>
                        )}
                        <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground/60">
                            <span className="font-mono bg-muted/40 px-1.5 py-0.5 rounded">{(wf.id ?? "").slice(0, 12)}</span>
                            <span>Criado em {new Date(wf.created_at || "").toLocaleString("pt-BR")}</span>
                        </div>
                    </div>
                    <div className="flex gap-2">
                        {(wf.status === "pending" || wf.status === "paused") && (
                            <Button size="sm" variant="default">
                                <Play className="w-3.5 h-3.5 mr-1.5" />
                                Executar
                            </Button>
                        )}
                        <Button size="sm" variant="ghost" onClick={onRefresh}>
                            <RefreshCw className="w-3.5 h-3.5" />
                        </Button>
                    </div>
                </div>

                <div className="grid grid-cols-4 gap-3 mt-4">
                    <MiniStat label="Etapas" value={wf.steps?.length ?? 0} />
                    <MiniStat label="Concluídas" value={wf.steps?.filter(s => s.status === "done" || s.status === "completed").length ?? 0} />
                    <MiniStat label="Estado" value={Object.keys(wf.state || {}).length + " chaves"} />
                    <MiniStat label="Status" value={wf.status ?? "—"} />
                </div>
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-6">
                {wf.status === "running" && (
                    <div className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400 text-sm">
                        <Activity className="w-4 h-4 animate-pulse" />
                        <span>Workflow em execução…</span>
                    </div>
                )}
                {wf.status === "failed" && (
                    <div className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
                        <AlertCircle className="w-4 h-4" />
                        <span>Workflow falhou</span>
                    </div>
                )}

                <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3 flex items-center gap-2">
                        <Terminal className="w-3.5 h-3.5" />
                        Etapas de execução
                    </h3>
                    <div className="rounded-xl border border-border/50 overflow-hidden bg-card/30">
                        {wf.steps?.map((step, i) => (
                            <StepRow
                                key={step.id || i}
                                step={step}
                                index={i}
                                isLast={i === (wf.steps?.length ?? 0) - 1}
                            />
                        ))}
                        {(!wf.steps || wf.steps.length === 0) && (
                            <div className="p-6 text-center text-sm text-muted-foreground">Nenhuma etapa definida</div>
                        )}
                    </div>
                </section>

                {wf.state && Object.keys(wf.state).length > 0 && (
                    <section>
                        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3 flex items-center gap-2">
                            <Activity className="w-3.5 h-3.5" />
                            Estado compartilhado
                        </h3>
                        <div className="rounded-xl border border-border/50 overflow-hidden bg-card/30">
                            <pre className="p-4 text-xs font-mono text-foreground/80 overflow-auto max-h-48 whitespace-pre-wrap">
                                {JSON.stringify(wf.state, null, 2)}
                            </pre>
                        </div>
                    </section>
                )}
            </div>
        </div>
    )
}

function StepRow({ step, index, isLast }: { step: WorkflowStep; index: number; isLast: boolean }) {
    const [open, setOpen] = useState(false)

    return (
        <div className={cn(!isLast && "border-b border-border/40")}>
            <button
                className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-accent/20 transition-colors group"
                onClick={() => setOpen(!open)}
            >
                <StatusIcon status={step.status} />
                <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between">
                        <p className="text-sm font-medium">
                            <span className="text-muted-foreground mr-1.5">#{index + 1}</span>
                            <span className="font-mono text-xs">{step.id ?? `step-${index}`}</span>
                        </p>
                        <span className="text-[10px] text-muted-foreground capitalize px-2 py-0.5 rounded-full bg-muted/40">
                            {step.status ?? "pending"}
                        </span>
                    </div>
                    {step.prompt && (
                        <p className="text-xs text-muted-foreground mt-0.5 truncate">{step.prompt}</p>
                    )}
                </div>
                {(step.result || (step.tools?.length ?? 0) > 0) && (
                    <ChevronRight className={cn("w-3.5 h-3.5 text-muted-foreground transition-transform flex-shrink-0", open && "rotate-90")} />
                )}
            </button>

            {open && (
                <div className="px-4 pb-3 space-y-2">
                    {(step.tools?.length ?? 0) > 0 && (
                        <div className="flex flex-wrap gap-1.5">
                            {step.tools!.map(t => (
                                <span key={t} className="inline-flex items-center gap-1 text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary border border-primary/20">
                                    <Zap className="w-2.5 h-2.5" />
                                    {t}
                                </span>
                            ))}
                        </div>
                    )}
                    {step.result?.output && (
                        <pre className="p-3 rounded-lg bg-muted/40 border border-border/50 text-xs font-mono text-foreground/80 overflow-auto max-h-40 whitespace-pre-wrap">
                            {step.result.output}
                        </pre>
                    )}
                </div>
            )}
        </div>
    )
}

function StatusBadge({ status }: { status?: string }) {
    const styles: Record<string, string> = {
        pending:   "bg-muted/50 text-muted-foreground border-border/50",
        running:   "bg-blue-500/10 text-blue-400 border-blue-500/20",
        completed: "bg-green-500/10 text-green-400 border-green-500/20",
        done:      "bg-green-500/10 text-green-400 border-green-500/20",
        failed:    "bg-red-500/10 text-red-400 border-red-500/20",
        paused:    "bg-amber-500/10 text-amber-400 border-amber-500/20",
        cancelled: "bg-muted/40 text-muted-foreground border-border/40",
    }
    return (
        <span className={cn("inline-flex items-center text-[11px] font-medium px-2 py-0.5 rounded-full border", styles[status ?? "pending"] ?? styles.pending)}>
            {status}
        </span>
    )
}

function StatusDot({ status }: { status?: string }) {
    const colors: Record<string, string> = {
        pending: "bg-muted-foreground/40",
        running: "bg-blue-400 animate-pulse",
        completed: "bg-green-400",
        done: "bg-green-400",
        failed: "bg-red-400",
        paused: "bg-amber-400",
        cancelled: "bg-muted-foreground/30",
    }
    return <div className={cn("w-2 h-2 rounded-full flex-shrink-0", colors[status ?? "pending"] ?? colors.pending)} />
}

function StatusIcon({ status }: { status?: string }) {
    if (status === "running")                         return <Activity     className="w-4 h-4 text-blue-400 animate-pulse flex-shrink-0" />
    if (status === "completed" || status === "done")  return <CheckCircle2 className="w-4 h-4 text-green-400 flex-shrink-0" />
    if (status === "failed")                          return <XCircle      className="w-4 h-4 text-red-400 flex-shrink-0" />
    if (status === "paused")                          return <Pause        className="w-4 h-4 text-amber-400 flex-shrink-0" />
    return <Clock className="w-4 h-4 text-muted-foreground/40 flex-shrink-0" />
}

function MiniStat({ label, value }: { label: string; value: string | number }) {
    return (
        <div className="rounded-lg bg-muted/30 border border-border/40 px-3 py-2">
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground/60 font-medium mb-0.5">{label}</p>
            <p className="text-sm font-semibold tabular-nums">{value}</p>
        </div>
    )
}
