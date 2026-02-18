import { useEffect } from "react"
import {
    FolderKanban, ImageIcon, Bot, Clock, Plus, ArrowRight,
    Activity, Brain, Wrench, CheckCircle, XCircle, Loader2,
    ScanSearch, Zap, GitBranch, MessageSquare,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { useProjectStore, type Project } from "@/store/projects"
import { useUIStore } from "@/store/ui"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

const API_BASE = "http://localhost:8080"

interface TraceSummary {
    id: string
    name: string
    status: "running" | "ok" | "error" | "cancelled"
    start_time: string
    duration_ms: number
    span_count: number
}

export function Dashboard() {
    const { projects, artifacts, fetchProjects, fetchArtifacts, createProject } = useProjectStore()
    const { openProject, setActiveView, toggleChatWindow } = useUIStore()

    useEffect(() => {
        fetchProjects()
        fetchArtifacts()
    }, [fetchProjects, fetchArtifacts])

    const { data: jobs } = useQuery({
        queryKey: ["jobs"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/jobs")
            if (error) throw error
            return data
        },
        refetchInterval: 10000,
    })

    const { data: tracesData } = useQuery<{ traces: TraceSummary[]; count: number }>({
        queryKey: ["traces-dashboard"],
        queryFn: async () => {
            const res = await fetch(`${API_BASE}/v1/traces?limit=6`)
            if (!res.ok) return { traces: [], count: 0 }
            return res.json()
        },
        refetchInterval: 5000,
    })

    const handleNewProject = async () => {
        const proj = await createProject("Untitled Project")
        if (proj) openProject(proj.id)
    }

    const recentArtifacts = artifacts.slice(0, 8)
    const recentJobs = (Array.isArray(jobs) ? jobs : []).slice(0, 5)
    const recentTraces = tracesData?.traces ?? []
    const runningTraces = recentTraces.filter(t => t.status === "running")

    return (
        <div className="h-full overflow-y-auto p-6 space-y-8">
            {/* Hero */}
            <div className="flex items-start justify-between">
                <div className="space-y-1">
                    <h1 className="text-2xl font-bold tracking-tight">auleOS</h1>
                    <p className="text-sm text-muted-foreground">
                        Workspace agêntico. Projetos, agentes, workflows e artefatos num só lugar.
                    </p>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    className="gap-2 text-xs"
                    onClick={toggleChatWindow}
                >
                    <MessageSquare className="w-3.5 h-3.5" />
                    Abrir Chat
                </Button>
            </div>

            {/* Active agent indicator */}
            {runningTraces.length > 0 && (
                <div className="flex items-center gap-3 px-4 py-3 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400 text-sm">
                    <Activity className="w-4 h-4 animate-pulse flex-shrink-0" />
                    <span className="font-medium">
                        {runningTraces.length} agente{runningTraces.length > 1 ? "s" : ""} em execução
                    </span>
                    <span className="text-blue-300/70 text-xs truncate">{runningTraces[0].name}</span>
                    <button
                        onClick={() => setActiveView("traces")}
                        className="ml-auto text-xs underline underline-offset-2 hover:text-blue-300 flex-shrink-0"
                    >
                        Ver traces
                    </button>
                </div>
            )}

            {/* Quick Stats */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                <StatCard icon={FolderKanban} label="Projetos" value={projects.length} onClick={() => setActiveView("project")} />
                <StatCard icon={ImageIcon}    label="Artefatos" value={artifacts.length} onClick={() => setActiveView("jobs")} />
                <StatCard icon={GitBranch}    label="Workflows" value={0} onClick={() => setActiveView("workflows")} />
                <StatCard icon={Clock}        label="Jobs" value={recentJobs.length} onClick={() => setActiveView("jobs")} />
            </div>

            {/* Projects */}
            <section>
                <div className="flex items-center justify-between mb-3">
                    <h2 className="font-semibold text-sm flex items-center gap-2">
                        <FolderKanban className="w-4 h-4" />
                        Projetos
                    </h2>
                    <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={handleNewProject}>
                        <Plus className="w-3 h-3" /> Novo
                    </Button>
                </div>

                {projects.length === 0 ? (
                    <div className="border border-dashed border-border/80 rounded-xl p-8 text-center text-muted-foreground text-sm">
                        <FolderKanban className="w-8 h-8 mx-auto mb-2 opacity-40" />
                        <p>Nenhum projeto ainda</p>
                        <Button variant="outline" size="sm" className="mt-3 text-xs" onClick={handleNewProject}>
                            Criar primeiro projeto
                        </Button>
                    </div>
                ) : (
                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                        {projects.slice(0, 6).map((proj) => (
                            <ProjectCard key={proj.id} project={proj} onClick={() => openProject(proj.id)} />
                        ))}
                    </div>
                )}
            </section>

            {/* Agent Activity (Traces) */}
            <section>
                <div className="flex items-center justify-between mb-3">
                    <h2 className="font-semibold text-sm flex items-center gap-2">
                        <ScanSearch className="w-4 h-4" />
                        Atividade do Agente
                        {runningTraces.length > 0 && (
                            <span className="flex h-2 w-2 rounded-full bg-blue-400 animate-pulse" />
                        )}
                    </h2>
                    <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={() => setActiveView("traces")}>
                        Ver traces <ArrowRight className="w-3 h-3" />
                    </Button>
                </div>

                {recentTraces.length === 0 ? (
                    <div className="border border-dashed border-border/60 rounded-xl p-6 text-center text-muted-foreground text-sm">
                        <Bot className="w-8 h-8 mx-auto mb-2 opacity-20" />
                        <p className="text-xs">Nenhuma atividade ainda. Inicie um chat para ver os traces aqui.</p>
                    </div>
                ) : (
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
                        {recentTraces.map(trace => (
                            <TraceCard key={trace.id} trace={trace} onClick={() => setActiveView("traces")} />
                        ))}
                    </div>
                )}
            </section>

            {/* Recent Artifacts */}
            {recentArtifacts.length > 0 && (
                <section>
                    <div className="flex items-center justify-between mb-3">
                        <h2 className="font-semibold text-sm flex items-center gap-2">
                            <ImageIcon className="w-4 h-4" />
                            Artefatos Recentes
                        </h2>
                        <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={() => setActiveView("jobs")}>
                            Ver todos <ArrowRight className="w-3 h-3" />
                        </Button>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        {recentArtifacts.map((art) => (
                            <ArtifactThumb key={art.id} artifact={art} />
                        ))}
                    </div>
                </section>
            )}
        </div>
    )
}

function StatCard({ icon: Icon, label, value, onClick }: {
    icon: React.ElementType
    label: string
    value: number
    onClick?: () => void
}) {
    return (
        <button
            onClick={onClick}
            className="flex items-center gap-3 p-3 rounded-xl bg-card/60 border border-border/50 hover:bg-accent/50 transition-colors text-left"
        >
            <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center">
                <Icon className="w-4 h-4 text-primary" />
            </div>
            <div>
                <p className="text-lg font-semibold leading-none">{value}</p>
                <p className="text-xs text-muted-foreground mt-0.5">{label}</p>
            </div>
        </button>
    )
}

function TraceCard({ trace, onClick }: { trace: TraceSummary; onClick: () => void }) {
    const isRunning = trace.status === "running"
    const isError   = trace.status === "error"
    const isOk      = trace.status === "ok"

    const StatusIconComp = isRunning ? Loader2 : isError ? XCircle : isOk ? CheckCircle : Clock
    const statusColor = isRunning ? "text-blue-400" : isError ? "text-red-400" : isOk ? "text-green-400" : "text-muted-foreground"

    // Detect span kinds from name patterns for visual hints
    const hasLLM  = trace.name.toLowerCase().includes("chat") || trace.span_count > 2
    const hasTool = trace.span_count > 3

    return (
        <button
            onClick={onClick}
            className={cn(
                "w-full text-left rounded-xl border px-4 py-3 transition-all hover:bg-accent/30 group",
                isRunning ? "bg-blue-500/5 border-blue-500/20" :
                isError   ? "bg-red-500/5 border-red-500/20" :
                "bg-card/50 border-border/40"
            )}
        >
            <div className="flex items-center gap-2 mb-2">
                <StatusIconComp className={cn("w-3.5 h-3.5 flex-shrink-0", statusColor, isRunning && "animate-spin")} />
                <span className="text-sm font-medium truncate flex-1 group-hover:text-foreground">{trace.name}</span>
                <span className={cn("text-[10px] font-mono", statusColor)}>{trace.status}</span>
            </div>
            <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
                <span className="flex items-center gap-1">
                    <Zap className="w-3 h-3" />
                    {trace.span_count} spans
                </span>
                <span className="flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {trace.duration_ms > 1000 ? `${(trace.duration_ms / 1000).toFixed(1)}s` : `${trace.duration_ms}ms`}
                </span>
                <div className="flex items-center gap-1 ml-auto">
                    {hasLLM  && <span className="px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-400 text-[10px]"><Brain className="w-3 h-3 inline mr-0.5" />LLM</span>}
                    {hasTool && <span className="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 text-[10px]"><Wrench className="w-3 h-3 inline mr-0.5" />Tool</span>}
                </div>
            </div>
            <p className="text-[10px] text-muted-foreground/50 mt-1.5">
                {new Date(trace.start_time).toLocaleString("pt-BR")}
            </p>
        </button>
    )
}

function ProjectCard({ project, onClick }: { project: Project; onClick: () => void }) {
    return (
        <button
            onClick={onClick}
            className="text-left p-4 rounded-xl bg-card/60 border border-border/50 hover:border-primary/30 hover:bg-accent/30 transition-all group"
        >
            <div className="flex items-center gap-2 mb-1">
                <FolderKanban className="w-4 h-4 text-primary/60 group-hover:text-primary transition-colors" />
                <h3 className="font-medium text-sm truncate">{project.name}</h3>
            </div>
            {project.description && (
                <p className="text-xs text-muted-foreground line-clamp-2">{project.description}</p>
            )}
            <p className="text-[10px] text-muted-foreground/60 mt-2">
                {new Date(project.updated_at).toLocaleDateString()}
            </p>
        </button>
    )
}

function ArtifactThumb({ artifact }: { artifact: { id: string; name: string; type: string; file_path: string; mime_type: string } }) {
    const isImage = artifact.type === "image" || artifact.mime_type.startsWith("image/")

    // Build URL for serving the artifact file
    const getArtifactUrl = () => {
        // file_path is like /home/gohan/auleOS/workspace/jobs/{jobId}/output.png
        // We need to extract jobId and filename
        const parts = artifact.file_path.split("/")
        const filename = parts[parts.length - 1]
        const jobId = parts[parts.length - 2]
        return `http://localhost:8080/v1/jobs/${jobId}/files/${filename}`
    }

    return (
        <div className="rounded-xl bg-card/60 border border-border/50 overflow-hidden group hover:border-primary/30 transition-all">
            {isImage ? (
                <div className="aspect-square bg-muted/30 relative overflow-hidden">
                    <img
                        src={getArtifactUrl()}
                        alt={artifact.name}
                        className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                        loading="lazy"
                    />
                </div>
            ) : (
                <div className="aspect-square bg-muted/30 flex items-center justify-center">
                    <ImageIcon className="w-8 h-8 text-muted-foreground/30" />
                </div>
            )}
            <div className="p-2">
                <p className="text-xs font-medium truncate">{artifact.name}</p>
                <p className="text-[10px] text-muted-foreground capitalize">{artifact.type}</p>
            </div>
        </div>
    )
}
