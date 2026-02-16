import { Button } from "@/components/ui/button"
import { GlassPanel } from "@/components/ui/glass-panel"
import { Plus, Settings, Loader2 } from "lucide-react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

interface SidebarProps {
    currentJobId?: string | null;
    onSelectJob?: (id: string) => void;
}

export function Sidebar({ currentJobId, onSelectJob }: SidebarProps) {
    const queryClient = useQueryClient();

    // 1. Fetch Jobs
    const { data: jobs, isLoading } = useQuery({
        queryKey: ["jobs"],
        queryFn: async () => {
            const { data, error } = await api.GET("/v1/jobs");
            if (error) throw new Error(JSON.stringify(error));
            return data; // This should be Job[]
        },
        refetchInterval: 5000,
    });

    // 2. Create Job Mutation
    const createJob = useMutation({
        mutationFn: async () => {
            const { data, error } = await api.POST("/v1/jobs", {
                body: {
                    image: "alpine",
                    command: ["echo", "hello world from frontend"],
                    // Default demo job
                }
            });
            if (error) throw new Error(JSON.stringify(error));
            return data;
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ["jobs"] });
            if (data && data.id && onSelectJob) onSelectJob(data.id);
        }
    });

    return (
        <GlassPanel className="h-full flex flex-col gap-4 p-4 rounded-2xl" intensity="md">
            <div className="flex items-center justify-between px-2">
                <h2 className="text-sm font-semibold tracking-tight">Jobs</h2>
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={() => createJob.mutate()}
                    disabled={createJob.isPending}
                >
                    {createJob.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                </Button>
            </div>

            <div className="flex-1 overflow-y-auto space-y-2">
                {isLoading && (
                    <div className="flex justify-center p-4">
                        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                    </div>
                )}

                {Array.isArray(jobs) && jobs.map((job) => (
                    <div
                        key={job.id}
                        onClick={() => job.id && onSelectJob?.(job.id)}
                        className={cn(
                            "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium cursor-pointer transition-colors",
                            job.id && currentJobId === job.id ? "bg-primary/10 text-primary border border-primary/20" : "hover:bg-accent hover:text-accent-foreground"
                        )}
                    >
                        <div className={cn(
                            "h-2 w-2 rounded-full",
                            job.status === "RUNNING" ? "bg-green-500 animate-pulse" :
                                job.status === "QUEUED" ? "bg-amber-500" :
                                job.status === "COMPLETED" ? "bg-blue-500" :
                                    job.status === "FAILED" ? "bg-red-500" : "bg-gray-400"
                        )} />
                        <span className="truncate flex-1">{(job.id || "unknown").substring(0, 8)}...</span>
                        <span className="text-xs text-muted-foreground opacity-50">{job.status}</span>
                    </div>
                ))}

                {!isLoading && Array.isArray(jobs) && jobs.length === 0 && (
                    <div className="text-center p-4 text-xs text-muted-foreground">
                        No jobs found.
                    </div>
                )}
            </div>

            <div className="mt-auto pt-4 border-t border-border/50">
                <Button variant="ghost" className="w-full justify-start gap-3">
                    <Settings className="h-4 w-4" />
                    <span>Settings</span>
                </Button>
            </div>
        </GlassPanel>
    )
}
