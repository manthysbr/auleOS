import { useEffect } from "react"
import { FolderKanban, ImageIcon, Bot, Clock, Plus, ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useProjectStore, type Project } from "@/store/projects"
import { useUIStore } from "@/store/ui"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"

export function Dashboard() {
    const { projects, artifacts, fetchProjects, fetchArtifacts, createProject } = useProjectStore()
    const { openProject, setActiveView } = useUIStore()

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

    const handleNewProject = async () => {
        const proj = await createProject("Untitled Project")
        if (proj) {
            openProject(proj.id)
        }
    }

    const recentArtifacts = artifacts.slice(0, 8)
    const recentJobs = (Array.isArray(jobs) ? jobs : []).slice(0, 5)

    return (
        <div className="h-full overflow-y-auto p-6 space-y-8">
            {/* Hero */}
            <div className="space-y-1">
                <h1 className="text-2xl font-bold tracking-tight">Welcome to auleOS</h1>
                <p className="text-sm text-muted-foreground">
                    Your agentic creative workspace. Open a project, explore artifacts, or start a chat.
                </p>
            </div>

            {/* Quick Stats */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                <StatCard
                    icon={FolderKanban}
                    label="Projects"
                    value={projects.length}
                    onClick={() => setActiveView("project")}
                />
                <StatCard
                    icon={ImageIcon}
                    label="Artifacts"
                    value={artifacts.length}
                    onClick={() => setActiveView("jobs")}
                />
                <StatCard
                    icon={Bot}
                    label="Agents"
                    value={1}
                    onClick={() => setActiveView("agents")}
                />
                <StatCard
                    icon={Clock}
                    label="Jobs"
                    value={recentJobs.length}
                    onClick={() => setActiveView("jobs")}
                />
            </div>

            {/* Projects */}
            <section>
                <div className="flex items-center justify-between mb-3">
                    <h2 className="font-semibold text-sm flex items-center gap-2">
                        <FolderKanban className="w-4 h-4" />
                        Projects
                    </h2>
                    <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={handleNewProject}>
                        <Plus className="w-3 h-3" /> New
                    </Button>
                </div>

                {projects.length === 0 ? (
                    <div className="border border-dashed border-border/80 rounded-xl p-8 text-center text-muted-foreground text-sm">
                        <FolderKanban className="w-8 h-8 mx-auto mb-2 opacity-40" />
                        <p>No projects yet</p>
                        <Button variant="outline" size="sm" className="mt-3 text-xs" onClick={handleNewProject}>
                            Create your first project
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

            {/* Recent Artifacts */}
            {recentArtifacts.length > 0 && (
                <section>
                    <div className="flex items-center justify-between mb-3">
                        <h2 className="font-semibold text-sm flex items-center gap-2">
                            <ImageIcon className="w-4 h-4" />
                            Recent Artifacts
                        </h2>
                        <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={() => setActiveView("jobs")}>
                            View All <ArrowRight className="w-3 h-3" />
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
