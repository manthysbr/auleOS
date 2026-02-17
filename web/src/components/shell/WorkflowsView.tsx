import { useState, useEffect } from "react"
import type { components } from "@/lib/api.schema"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Play, Pause, Clock, CheckCircle2, XCircle, Layout, Activity, Terminal } from "lucide-react"

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
            if (error) {
                console.error("Failed to fetch workflows:", error)
                return
            }
            if (data) {
                setWorkflows(data)
            }
        } catch (e) {
            console.error("Error fetching workflows:", e)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchWorkflows()
    }, [])

    const selectedWorkflow = workflows.find(w => w.id === selectedId)

    return (
        <div className="flex h-full bg-slate-50 dark:bg-slate-900">
            {/* Sidebar List */}
            <div className="w-80 border-r border-slate-200 dark:border-slate-800 flex flex-col bg-white dark:bg-slate-950">
                <div className="p-4 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
                    <h2 className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                        <Layout className="w-4 h-4 text-slate-500" />
                        Workflows
                    </h2>
                    <Button variant="ghost" size="icon" onClick={fetchWorkflows} disabled={loading}>
                        <Activity className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />
                    </Button>
                </div>
                <div className="flex-1 overflow-y-auto">
                    <div className="p-2 space-y-2">
                        {workflows.map(wf => (
                            <div
                                key={wf.id}
                                onClick={() => setSelectedId(wf.id ?? null)}
                                className={`
                  p-3 rounded-lg cursor-pointer transition-colors border text-left
                  ${selectedId === wf.id
                                        ? "bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700"
                                        : "bg-white dark:bg-slate-950 border-transparent hover:bg-slate-50 dark:hover:bg-slate-900"}
                `}
                            >
                                <div className="flex justify-between items-start mb-1">
                                    <span className="font-medium text-sm truncate pr-2">{wf.name}</span>
                                    <StatusBadge status={wf.status} size="sm" />
                                </div>
                                {wf.description && (
                                    <p className="text-xs text-slate-500 line-clamp-2 mb-2">{wf.description}</p>
                                )}
                                <div className="flex items-center gap-2 text-[10px] text-slate-400">
                                    <Clock className="w-3 h-3" />
                                    <span>{new Date(wf.created_at || "").toLocaleDateString()}</span>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>

            {/* Main Content Area */}
            <div className="flex-1 flex flex-col overflow-hidden">
                {selectedWorkflow ? (
                    <WorkflowDetail wf={selectedWorkflow} />
                ) : (
                    <div className="flex-1 flex flex-col items-center justify-center text-slate-400">
                        <Layout className="w-16 h-16 mb-4 opacity-20" />
                        <p>Select a workflow to view details</p>
                    </div>
                )}
            </div>
        </div>
    )
}

function WorkflowDetail({ wf }: { wf: Workflow }) {
    return (
        <div className="flex-1 flex flex-col h-full overflow-hidden">
            <div className="p-6 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950">
                <div className="flex justify-between items-start mb-4">
                    <div>
                        <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">{wf.name}</h1>
                        <div className="flex items-center gap-3 mt-2 text-sm text-slate-500">
                            <StatusBadge status={wf.status} />
                            <span>•</span>
                            <span className="font-mono text-xs bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded">
                                {(wf.id ?? "").slice(0, 8)}
                            </span>
                        </div>
                    </div>
                    <div className="flex gap-2">
                        {/* Actions like Run/Resume would go here */}
                        {wf.status === "pending" || wf.status === "paused" ? (
                            <Button size="sm">
                                <Play className="w-4 h-4 mr-2" />
                                Run
                            </Button>
                        ) : null}
                    </div>
                </div>
                {wf.description && (
                    <p className="text-slate-600 dark:text-slate-400 max-w-3xl">{wf.description}</p>
                )}

                <div className="grid grid-cols-4 gap-4 mt-6">
                    <StatCard label="Created" value={new Date(wf.created_at || "").toLocaleString()} />
                    <StatCard label="Steps" value={wf.steps?.length || 0} />
                    <StatCard label="State Keys" value={Object.keys(wf.state || {}).length} />
                    <StatCard label="ID short" value={(wf.id || "").slice(0, 8)} fontMono />
                </div>
            </div>

            <div className="flex-1 overflow-y-auto bg-slate-50 dark:bg-slate-900 p-6">
                <div className="max-w-4xl mx-auto space-y-8">
                    {/* Steps Visualization */}
                    <section>
                        <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-4 flex items-center gap-2">
                            <Terminal className="w-4 h-4" />
                            Execution Steps
                        </h3>
                        <div className="bg-white dark:bg-slate-950 rounded-lg border border-slate-200 dark:border-slate-800 overflow-hidden shadow-sm">
                            <div className="p-0">
                                {wf.steps?.map((step, i) => (
                                    <StepRow key={step.id || i} step={step} index={i} isLast={i === (wf.steps?.length || 0) - 1} />
                                ))}
                            </div>
                        </div>
                    </section>

                    {/* Shared State View */}
                    <section>
                        <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-4 flex items-center gap-2">
                            <Activity className="w-4 h-4" />
                            Shared State
                        </h3>
                        <div className="bg-white dark:bg-slate-950 rounded-lg border border-slate-200 dark:border-slate-800 overflow-hidden shadow-sm">
                            <div className="p-4">
                                <pre className="text-xs bg-slate-950 text-slate-50 p-4 rounded-md overflow-auto font-mono">
                                    {JSON.stringify(wf.state, null, 2)}
                                </pre>
                            </div>
                        </div>
                    </section>
                </div>
            </div>
        </div>
    )
}

function StepRow({ step, index, isLast }: { step: WorkflowStep; index: number; isLast: boolean }) {
    // using 'done' as per generated schema

    return (
        <div className={`flex gap-4 p-4 ${!isLast ? "border-b border-slate-100 dark:border-slate-800" : ""}`}>
            <div className="pt-1">
                <StatusIcon status={step.status} />
            </div>
            <div className="flex-1 min-w-0">
                <div className="flex justify-between items-start mb-1">
                    <p className="font-medium text-sm text-slate-900 dark:text-slate-100">
                        Step {index + 1}: {(step.id ?? "").slice(0, 8)}
                    </p>
                    <span className="text-xs text-slate-400 capitalize">{step.status}</span>
                </div>
                <p className="text-sm text-slate-600 dark:text-slate-400 mb-2 font-mono text-xs bg-slate-50 dark:bg-slate-900 p-2 rounded border border-slate-100 dark:border-slate-800">
                    {step.prompt}
                </p>
                {step.tools && step.tools.length > 0 && (
                    <div className="flex gap-2 mb-2">
                        {step.tools.map(t => (
                            <span key={t} className="inline-flex items-center rounded-md border border-slate-200 px-2 py-0.5 text-[10px] font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 text-foreground">
                                {t}
                            </span>
                        ))}
                    </div>
                )}
                {step.result && (
                    <div className="bg-slate-100 dark:bg-slate-900 p-2 rounded text-xs font-mono text-slate-700 dark:text-slate-300 whitespace-pre-wrap border border-slate-200 dark:border-slate-800">
                        {step.result.output}
                    </div>
                )}
            </div>
        </div>
    )
}

function StatusBadge({ status, size = "md" }: { status?: string; size?: "sm" | "md" }) {
    const colors: Record<string, string> = {
        pending: "bg-slate-100 text-slate-600 border-slate-200",
        running: "bg-blue-50 text-blue-600 border-blue-200",
        completed: "bg-green-50 text-green-600 border-green-200", // Workflow status
        done: "bg-green-50 text-green-600 border-green-200",      // Step status
        failed: "bg-red-50 text-red-600 border-red-200",
        paused: "bg-amber-50 text-amber-600 border-amber-200",
        cancelled: "bg-slate-100 text-slate-500 border-slate-200",
        skipped: "bg-slate-50 text-slate-400 border-slate-200",
    }
    const colorClass = colors[status || "pending"] || colors.pending
    const sizeClass = size === "sm" ? "px-1.5 py-0.5 text-[10px]" : "px-2.5 py-0.5 text-xs"

    return (
        <span className={`inline-flex items-center font-medium rounded-full border ${colorClass} ${sizeClass}`}>
            {status}
        </span>
    )
}

function StatusIcon({ status }: { status?: string }) {
    if (status === "running") return <Activity className="w-5 h-5 text-blue-500 animate-pulse" />
    if (status === "completed" || status === "done") return <CheckCircle2 className="w-5 h-5 text-green-500" />
    if (status === "failed") return <XCircle className="w-5 h-5 text-red-500" />
    if (status === "paused") return <Pause className="w-5 h-5 text-amber-500" />
    return <Clock className="w-5 h-5 text-slate-300" />
}

function StatCard({ label, value, fontMono }: { label: string; value: string | number; fontMono?: boolean }) {
    return (
        <div className="p-3 bg-slate-50 dark:bg-slate-900 rounded border border-slate-100 dark:border-slate-800">
            <p className="text-[10px] uppercase tracking-wider text-slate-500 font-semibold mb-1">{label}</p>
            <p className={`text-sm font-medium ${fontMono ? "font-mono" : ""}`}>{value}</p>
        </div>
    )
}
