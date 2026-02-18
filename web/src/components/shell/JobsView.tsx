import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Activity, Clock, CheckCircle, XCircle, Loader2, AlertCircle, Image, X, ZoomIn, Plus, ChevronDown, ChevronUp, Play } from "lucide-react"
import { cn } from "@/lib/utils"

const statusIcon: Record<string, React.ElementType> = {
    QUEUED: Clock,
    RUNNING: Loader2,
    COMPLETED: CheckCircle,
    FAILED: XCircle,
    CANCELLED: AlertCircle,
}

const statusColor: Record<string, string> = {
    QUEUED: "text-yellow-500",
    RUNNING: "text-blue-500",
    COMPLETED: "text-green-500",
    FAILED: "text-red-500",
    CANCELLED: "text-muted-foreground",
}

function isImageResult(result?: string): boolean {
    if (!result) return false
    return /\.(png|jpg|jpeg|gif|webp|svg)$/i.test(result.trim())
}

function ImagePreviewModal({ src, onClose }: { src: string; onClose: () => void }) {
    return (
        <div
            className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 backdrop-blur-sm"
            onClick={onClose}
        >
            <div
                className="relative max-w-[90vw] max-h-[90vh] rounded-2xl overflow-hidden shadow-2xl border border-border/30"
                onClick={e => e.stopPropagation()}
            >
                <button
                    onClick={onClose}
                    className="absolute top-2 right-2 z-10 w-8 h-8 rounded-full bg-black/60 flex items-center justify-center text-white hover:bg-black/80 transition-colors"
                >
                    <X className="w-4 h-4" />
                </button>
                <img
                    src={src}
                    alt="Job result"
                    className="max-w-full max-h-[90vh] object-contain"
                />
            </div>
        </div>
    )
}

export function JobsView() {
    const [previewSrc, setPreviewSrc] = useState<string | null>(null)
    const [showForm, setShowForm] = useState(false)
    const [formImage, setFormImage] = useState("")
    const [formCommand, setFormCommand] = useState("")
    const [formAgentPrompt, setFormAgentPrompt] = useState("")
    const [submitResult, setSubmitResult] = useState<{ id?: string; error?: string } | null>(null)
    const queryClient = useQueryClient()

    const submitMutation = useMutation({
        mutationFn: async () => {
            const cmd = formCommand.trim()
                ? formCommand.trim().match(/(?:[^\s"']+|"[^"]*"|'[^']*')+/g) ?? [formCommand.trim()]
                : []
            const env: Record<string, string> = {}
            if (formAgentPrompt.trim()) {
                env["AULE_AGENT_PROMPT"] = formAgentPrompt.trim()
            }
            const { data, error } = await api.POST("/v1/jobs", {
                body: {
                    image: formImage.trim(),
                    command: cmd,
                    ...(Object.keys(env).length > 0 ? { env } : {}),
                },
            })
            if (error) throw new Error(JSON.stringify(error))
            return data
        },
        onSuccess: (data) => {
            setSubmitResult({ id: (data as any)?.id ?? "submitted" })
            setFormImage("")
            setFormCommand("")
            setFormAgentPrompt("")
            queryClient.invalidateQueries({ queryKey: ["jobs"] })
        },
        onError: (err: Error) => {
            setSubmitResult({ error: err.message })
        },
    })

    const { data: jobs, isLoading } = useQuery({
        queryKey: ["jobs"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/jobs")
            if (error) throw error
            return data
        },
        refetchInterval: 5000,
    })

    const jobList = Array.isArray(jobs) ? jobs : []

    return (
        <div className="h-full overflow-y-auto p-6 space-y-4">
            {previewSrc && <ImagePreviewModal src={previewSrc} onClose={() => setPreviewSrc(null)} />}

            <div className="flex items-center gap-2">
                <Activity className="w-5 h-5 text-primary" />
                <h1 className="text-lg font-bold">Jobs</h1>
                <span className="text-sm text-muted-foreground">({jobList.length})</span>
                <button
                    onClick={() => { setShowForm(f => !f); setSubmitResult(null) }}
                    className="ml-auto flex items-center gap-1 text-xs px-2 py-1 rounded-lg bg-primary/10 text-primary hover:bg-primary/20 transition-colors"
                >
                    <Plus className="w-3 h-3" />
                    Submit Job
                    {showForm ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                </button>
            </div>

            {showForm && (
                <div className="p-4 rounded-xl bg-card/80 border border-border/60 space-y-3">
                    <p className="text-xs text-muted-foreground font-semibold uppercase tracking-wide">New Job</p>
                    <div className="space-y-2">
                        <label className="text-xs text-muted-foreground">Docker Image</label>
                        <input
                            type="text"
                            placeholder="e.g. alpine:latest"
                            value={formImage}
                            onChange={e => setFormImage(e.target.value)}
                            className="w-full px-3 py-1.5 text-sm rounded-lg bg-background border border-border/60 focus:outline-none focus:border-primary/60 font-mono"
                        />
                    </div>
                    <div className="space-y-2">
                        <label className="text-xs text-muted-foreground">Command (shell string)</label>
                        <input
                            type="text"
                            placeholder="e.g. echo hello world"
                            value={formCommand}
                            onChange={e => setFormCommand(e.target.value)}
                            className="w-full px-3 py-1.5 text-sm rounded-lg bg-background border border-border/60 focus:outline-none focus:border-primary/60 font-mono"
                            onKeyDown={e => { if (e.key === "Enter" && formImage.trim()) submitMutation.mutate() }}
                        />
                    </div>
                    <div className="space-y-2">
                        <label className="text-xs text-muted-foreground">
                            Agent Prompt <span className="opacity-60">(opcional — instrução para o container via AULE_AGENT_PROMPT)</span>
                        </label>
                        <textarea
                            rows={2}
                            placeholder="e.g. Analise os arquivos em /workspace e gere um relatório em /workspace/report.md"
                            value={formAgentPrompt}
                            onChange={e => setFormAgentPrompt(e.target.value)}
                            className="w-full px-3 py-1.5 text-sm rounded-lg bg-background border border-border/60 focus:outline-none focus:border-primary/60 resize-none"
                        />
                    </div>
                    <button
                        disabled={!formImage.trim() || submitMutation.isPending}
                        onClick={() => submitMutation.mutate()}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50 hover:bg-primary/90 transition-colors"
                    >
                        {submitMutation.isPending
                            ? <Loader2 className="w-3 h-3 animate-spin" />
                            : <Play className="w-3 h-3" />}
                        {submitMutation.isPending ? "Submitting…" : "Run Job"}
                    </button>
                    {submitResult?.id && (
                        <p className="text-xs text-green-500">Job submitted: <span className="font-mono">{submitResult.id}</span></p>
                    )}
                    {submitResult?.error && (
                        <p className="text-xs text-red-500">Error: {submitResult.error}</p>
                    )}
                </div>
            )}

            {isLoading && (
                <div className="flex justify-center p-8">
                    <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
                </div>
            )}

            {!isLoading && jobList.length === 0 && (
                <div className="text-center p-12 text-muted-foreground text-sm">
                    No jobs yet. Use the chat to trigger tools.
                </div>
            )}

            <div className="space-y-2">
                {jobList.map((job) => {
                    const status = job.status || "UNKNOWN"
                    const Icon = statusIcon[status] || AlertCircle
                    const color = statusColor[status] || "text-muted-foreground"
                    const hasImage = isImageResult(job.result ?? undefined)
                    return (
                        <div
                            key={job.id}
                            className="flex items-start gap-3 p-3 rounded-xl bg-card/60 border border-border/50 hover:bg-accent/30 transition-colors"
                        >
                            <Icon className={cn("w-4 h-4 flex-shrink-0 mt-0.5", color, status === "RUNNING" && "animate-spin")} />
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-mono truncate">{job.id}</p>
                                {/* Image thumbnail */}
                                {hasImage && job.result && (
                                    <div
                                        className="mt-2 relative group cursor-pointer w-20 h-20 rounded-lg overflow-hidden border border-border/50"
                                        onClick={() => setPreviewSrc(job.result ?? null)}
                                    >
                                        <img
                                            src={job.result}
                                            alt="Generated"
                                            className="w-full h-full object-cover"
                                            onError={e => { (e.target as HTMLImageElement).style.display = "none" }}
                                        />
                                        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/40 flex items-center justify-center transition-all">
                                            <ZoomIn className="w-5 h-5 text-white opacity-0 group-hover:opacity-100 transition-opacity" />
                                        </div>
                                    </div>
                                )}
                                {!hasImage && job.result && (
                                    <p className="text-xs text-muted-foreground truncate">{job.result}</p>
                                )}
                                {job.error && (
                                    <p className="text-xs text-red-500 truncate">{job.error}</p>
                                )}
                                {(job as any).metadata?.tools_used && (
                                    <p className="text-[10px] text-primary/70 truncate">
                                        tools: {(job as any).metadata.tools_used}
                                    </p>
                                )}
                            </div>
                            <div className="flex flex-col items-end gap-1 flex-shrink-0">
                                <span className={cn("text-xs font-medium uppercase", color)}>{status}</span>
                                {hasImage && job.result && (
                                    <button
                                        onClick={() => setPreviewSrc(job.result ?? null)}
                                        className="flex items-center gap-1 text-[10px] text-primary/70 hover:text-primary transition-colors"
                                    >
                                        <Image className="w-3 h-3" />
                                        view
                                    </button>
                                )}
                                {job.created_at && (
                                    <span className="text-[10px] text-muted-foreground">
                                        {new Date(job.created_at).toLocaleTimeString()}
                                    </span>
                                )}
                            </div>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}

