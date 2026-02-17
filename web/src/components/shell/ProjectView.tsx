import { useEffect, useState } from "react"
import { ArrowLeft, FolderKanban, ImageIcon, MessageSquare, Pencil, Trash2, Check, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useProjectStore, type Artifact } from "@/store/projects"
import { useUIStore } from "@/store/ui"
import { api } from "@/lib/api"

interface Conversation {
    id: string
    title: string
    created_at: string
}

export function ProjectView() {
    const { activeProjectId, setActiveView } = useUIStore()
    const { projects, updateProject, deleteProject, fetchProjectArtifacts } = useProjectStore()
    const project = projects.find((p) => p.id === activeProjectId)
    const [artifacts, setArtifacts] = useState<Artifact[]>([])
    const [conversations, setConversations] = useState<Conversation[]>([])
    const [editing, setEditing] = useState(false)
    const [editName, setEditName] = useState("")

    useEffect(() => {
        if (!activeProjectId) return

        fetchProjectArtifacts(activeProjectId).then(setArtifacts)

        api.GET("/v1/projects/{id}/conversations", {
            params: { path: { id: activeProjectId } },
        }).then(({ data }) => {
            if (data) setConversations((data as unknown as Conversation[]) ?? [])
        })
    }, [activeProjectId, fetchProjectArtifacts])

    if (!project) {
        return (
            <div className="h-full flex items-center justify-center text-muted-foreground">
                <p>Project not found</p>
            </div>
        )
    }

    const handleDelete = async () => {
        await deleteProject(project.id)
        setActiveView("dashboard")
    }

    const handleSaveName = async () => {
        if (editName.trim()) {
            await updateProject(project.id, editName.trim())
        }
        setEditing(false)
    }

    return (
        <div className="h-full overflow-y-auto p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center gap-3">
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={() => setActiveView("dashboard")}
                >
                    <ArrowLeft className="w-4 h-4" />
                </Button>
                <div className="flex items-center gap-2 flex-1 min-w-0">
                    <FolderKanban className="w-5 h-5 text-primary flex-shrink-0" />
                    {editing ? (
                        <div className="flex items-center gap-1">
                            <input
                                value={editName}
                                onChange={(e) => setEditName(e.target.value)}
                                className="text-lg font-bold bg-transparent border-b-2 border-primary focus:outline-none px-1"
                                autoFocus
                                onKeyDown={(e) => e.key === "Enter" && handleSaveName()}
                            />
                            <button onClick={handleSaveName} className="p-1 hover:text-primary">
                                <Check className="w-4 h-4" />
                            </button>
                            <button onClick={() => setEditing(false)} className="p-1 hover:text-destructive">
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                    ) : (
                        <h1
                            className="text-lg font-bold truncate cursor-pointer hover:text-primary transition-colors"
                            onClick={() => {
                                setEditName(project.name)
                                setEditing(true)
                            }}
                        >
                            {project.name}
                        </h1>
                    )}
                    {!editing && (
                        <button
                            onClick={() => {
                                setEditName(project.name)
                                setEditing(true)
                            }}
                            className="opacity-0 hover:opacity-100 text-muted-foreground hover:text-foreground transition-opacity"
                        >
                            <Pencil className="w-3 h-3" />
                        </button>
                    )}
                </div>
                <Button variant="ghost" size="sm" className="text-destructive hover:bg-destructive/10 text-xs" onClick={handleDelete}>
                    <Trash2 className="w-3 h-3 mr-1" /> Delete
                </Button>
            </div>

            {project.description && (
                <p className="text-sm text-muted-foreground">{project.description}</p>
            )}

            {/* Artifacts */}
            <section>
                <h2 className="font-semibold text-sm flex items-center gap-2 mb-3">
                    <ImageIcon className="w-4 h-4" />
                    Artifacts ({artifacts.length})
                </h2>
                {artifacts.length === 0 ? (
                    <div className="border border-dashed border-border/80 rounded-xl p-6 text-center text-muted-foreground text-sm">
                        No artifacts in this project yet.
                        <br />
                        <span className="text-xs">Generate content via chat and assign it here.</span>
                    </div>
                ) : (
                    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
                        {artifacts.map((art) => (
                            <ArtifactCard key={art.id} artifact={art} />
                        ))}
                    </div>
                )}
            </section>

            {/* Conversations */}
            <section>
                <h2 className="font-semibold text-sm flex items-center gap-2 mb-3">
                    <MessageSquare className="w-4 h-4" />
                    Conversations ({conversations.length})
                </h2>
                {conversations.length === 0 ? (
                    <div className="border border-dashed border-border/80 rounded-xl p-6 text-center text-muted-foreground text-sm">
                        No conversations linked to this project.
                    </div>
                ) : (
                    <div className="space-y-1">
                        {conversations.map((conv) => (
                            <div key={conv.id} className="flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-accent text-sm">
                                <MessageSquare className="w-3.5 h-3.5 text-muted-foreground" />
                                <span className="flex-1 truncate">{conv.title || "Untitled"}</span>
                                <span className="text-xs text-muted-foreground">
                                    {new Date(conv.created_at).toLocaleDateString()}
                                </span>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </div>
    )
}

function ArtifactCard({ artifact }: { artifact: Artifact }) {
    const isImage = artifact.type === "image" || artifact.mime_type.startsWith("image/")
    const parts = artifact.file_path.split("/")
    const filename = parts[parts.length - 1]
    const jobId = parts[parts.length - 2]
    const url = `http://localhost:8080/v1/jobs/${jobId}/files/${filename}`

    return (
        <div className="rounded-xl bg-card/60 border border-border/50 overflow-hidden group hover:border-primary/30 transition-all">
            {isImage ? (
                <div className="aspect-square bg-muted/30 relative overflow-hidden">
                    <img
                        src={url}
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
