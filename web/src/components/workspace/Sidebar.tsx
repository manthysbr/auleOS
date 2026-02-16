import { useEffect } from "react"
import { Button } from "@/components/ui/button"
import { GlassPanel } from "@/components/ui/glass-panel"
import { Plus, Settings, Loader2, MessageSquare, Trash2 } from "lucide-react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import { useConversationStore } from "@/store/conversations"

interface SidebarProps {
    currentJobId?: string | null
    onSelectJob?: (id: string) => void
    onOpenSettings?: () => void
}

export function Sidebar({ currentJobId, onSelectJob, onOpenSettings }: SidebarProps) {
    const queryClient = useQueryClient()

    const {
        conversations,
        activeConversationId,
        isLoadingConversations,
        fetchConversations,
        createConversation,
        selectConversation,
        deleteConversation,
        clearActive,
    } = useConversationStore()

    // Fetch conversations on mount
    useEffect(() => {
        fetchConversations()
    }, [fetchConversations])

    // 1. Fetch Jobs
    const { data: jobs, isLoading: isLoadingJobs } = useQuery({
        queryKey: ["jobs"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/jobs")
            if (error) throw new Error(JSON.stringify(error))
            return data
        },
        refetchInterval: 5000,
    })

    // 2. Create Job Mutation
    const createJob = useMutation({
        mutationFn: async () => {
            const { data, error } = await api.POST("/v1/jobs", {
                body: {
                    image: "alpine",
                    command: ["echo", "hello world from frontend"],
                },
            })
            if (error) throw new Error(JSON.stringify(error))
            return data
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["jobs"] })
            if (data && data.id && onSelectJob) onSelectJob(data.id)
        },
    })

    const handleNewChat = async () => {
        clearActive()
    }

    const handleSelectConversation = async (id: string) => {
        await selectConversation(id)
    }

    const handleDeleteConversation = async (e: React.MouseEvent, id: string) => {
        e.stopPropagation()
        await deleteConversation(id)
    }

    return (
        <GlassPanel className="h-full flex flex-col gap-2 p-4 rounded-2xl" intensity="md">
            {/* Conversations Section */}
            <div className="flex items-center justify-between px-2">
                <h2 className="text-sm font-semibold tracking-tight flex items-center gap-2">
                    <MessageSquare className="h-4 w-4" />
                    Chats
                </h2>
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={handleNewChat}
                    title="New Chat"
                >
                    <Plus className="h-4 w-4" />
                </Button>
            </div>

            <div className="flex-1 overflow-y-auto space-y-1">
                {isLoadingConversations && (
                    <div className="flex justify-center p-4">
                        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                    </div>
                )}

                {conversations.map((conv) => (
                    <div
                        key={conv.id}
                        onClick={() => handleSelectConversation(conv.id)}
                        className={cn(
                            "group flex items-center gap-2 rounded-lg px-3 py-2 text-sm cursor-pointer transition-colors",
                            activeConversationId === conv.id
                                ? "bg-primary/10 text-primary border border-primary/20"
                                : "hover:bg-accent hover:text-accent-foreground"
                        )}
                    >
                        <MessageSquare className="h-3.5 w-3.5 flex-shrink-0 opacity-50" />
                        <span className="truncate flex-1">{conv.title || "Untitled"}</span>
                        <button
                            onClick={(e) => handleDeleteConversation(e, conv.id)}
                            className="opacity-0 group-hover:opacity-100 hover:text-destructive transition-opacity p-0.5"
                            title="Delete"
                        >
                            <Trash2 className="h-3 w-3" />
                        </button>
                    </div>
                ))}

                {!isLoadingConversations && conversations.length === 0 && (
                    <div className="text-center p-4 text-xs text-muted-foreground">
                        No conversations yet. Start chatting!
                    </div>
                )}
            </div>

            {/* Jobs Section */}
            <div className="border-t border-border/50 pt-2">
                <div className="flex items-center justify-between px-2 mb-1">
                    <h2 className="text-xs font-semibold tracking-tight text-muted-foreground uppercase">Jobs</h2>
                    <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => createJob.mutate()}
                        disabled={createJob.isPending}
                    >
                        {createJob.isPending ? (
                            <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                            <Plus className="h-3 w-3" />
                        )}
                    </Button>
                </div>

                <div className="max-h-32 overflow-y-auto space-y-1">
                    {isLoadingJobs && (
                        <div className="flex justify-center p-2">
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        </div>
                    )}

                    {Array.isArray(jobs) &&
                        jobs.slice(0, 5).map((job) => (
                            <div
                                key={job.id}
                                onClick={() => job.id && onSelectJob?.(job.id)}
                                className={cn(
                                    "flex items-center gap-2 rounded-md px-2 py-1 text-xs cursor-pointer transition-colors",
                                    job.id && currentJobId === job.id
                                        ? "bg-primary/10 text-primary"
                                        : "hover:bg-accent hover:text-accent-foreground"
                                )}
                            >
                                <div
                                    className={cn(
                                        "h-1.5 w-1.5 rounded-full",
                                        job.status === "RUNNING"
                                            ? "bg-green-500 animate-pulse"
                                            : job.status === "QUEUED"
                                                ? "bg-amber-500"
                                                : job.status === "COMPLETED"
                                                    ? "bg-blue-500"
                                                    : job.status === "FAILED"
                                                        ? "bg-red-500"
                                                        : "bg-gray-400"
                                    )}
                                />
                                <span className="truncate flex-1">{(job.id || "?").substring(0, 8)}…</span>
                                <span className="text-[10px] text-muted-foreground opacity-50">{job.status}</span>
                            </div>
                        ))}
                </div>
            </div>

            {/* Settings */}
            <div className="mt-auto pt-2 border-t border-border/50">
                <Button variant="ghost" className="w-full justify-start gap-3" onClick={onOpenSettings}>
                    <Settings className="h-4 w-4" />
                    <span>Settings</span>
                </Button>
            </div>
        </GlassPanel>
    )
}
