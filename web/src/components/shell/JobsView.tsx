import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Activity, Clock, CheckCircle, XCircle, Loader2, AlertCircle } from "lucide-react"
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

export function JobsView() {
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
            <div className="flex items-center gap-2">
                <Activity className="w-5 h-5 text-primary" />
                <h1 className="text-lg font-bold">Jobs</h1>
                <span className="text-sm text-muted-foreground">({jobList.length})</span>
            </div>

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
                    return (
                        <div
                            key={job.id}
                            className="flex items-center gap-3 p-3 rounded-xl bg-card/60 border border-border/50 hover:bg-accent/30 transition-colors"
                        >
                            <Icon className={cn("w-4 h-4 flex-shrink-0", color, status === "RUNNING" && "animate-spin")} />
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-mono truncate">{job.id}</p>
                                {job.result && (
                                    <p className="text-xs text-muted-foreground truncate">{job.result}</p>
                                )}
                                {job.error && (
                                    <p className="text-xs text-red-500 truncate">{job.error}</p>
                                )}
                            </div>
                            <span className={cn("text-xs font-medium uppercase", color)}>{status}</span>
                            {job.created_at && (
                                <span className="text-[10px] text-muted-foreground">
                                    {new Date(job.created_at).toLocaleTimeString()}
                                </span>
                            )}
                        </div>
                    )
                })}
            </div>
        </div>
    )
}
