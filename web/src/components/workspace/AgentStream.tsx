import { GlassPanel } from "@/components/ui/glass-panel"
import { Bot, Cpu, Zap, Terminal } from "lucide-react"
import { useEventStream } from "@/hooks/useEventStream"
import { useEffect, useRef } from "react"
import { api } from "@/lib/api"
import { useQuery } from "@tanstack/react-query"

interface AgentStreamProps {
    jobId: string | null;
}

export function AgentStream({ jobId }: AgentStreamProps) {
    const { events, status } = useEventStream(jobId);
    const scrollRef = useRef<HTMLDivElement>(null);

    const { data: job } = useQuery({
        queryKey: ["job", jobId],
        queryFn: async () => {
            if (!jobId) return null
            const { data, error } = await api.GET("/v1/jobs/{id}", { params: { path: { id: jobId } } })
            if (error) throw new Error(JSON.stringify(error))
            return data
        },
        enabled: !!jobId,
        refetchInterval: 2000,
    })

    const getStatusLabel = (rawData: string) => {
        try {
            const parsed = JSON.parse(rawData) as { status?: string }
            if (parsed.status && parsed.status.trim() !== "") {
                return parsed.status
            }
        } catch {
            // Keep raw fallback when payload is not JSON
        }
        return rawData
    }

    const getStatusProgress = (rawData: string): number | null => {
        try {
            const parsed = JSON.parse(rawData) as { progress?: number }
            if (typeof parsed.progress === "number") {
                return Math.max(0, Math.min(100, parsed.progress))
            }
        } catch {
        }
        return null
    }

    const latestStatusEvent = [...events].reverse().find((event) => event.type === "status")
    const latestProgress = latestStatusEvent ? getStatusProgress(latestStatusEvent.data) : null
    const latestStatus = latestStatusEvent ? getStatusLabel(latestStatusEvent.data) : (job?.status || "UNKNOWN")

    const isImageResult = !!job?.result && /\.png($|\?)/i.test(job.result)
    const isTextResult = !!job?.result && /\.txt($|\?)/i.test(job.result)

    // Auto-scroll to bottom
    useEffect(() => {
        if (scrollRef.current) {
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        }
    }, [events]);

    if (!jobId) {
        return (
            <div className="h-full flex flex-col gap-4">
                <GlassPanel className="flex-1 p-6 rounded-2xl relative overflow-hidden flex flex-col items-center justify-center opacity-50" intensity="sm">
                    <Bot className="h-12 w-12 text-muted-foreground mb-4" />
                    <p>Select a job to view stream</p>
                </GlassPanel>
            </div>
        )
    }

    return (
        <div className="h-full flex flex-col gap-4">
            {/* Header / Status Bar */}
            <GlassPanel className="flex items-center justify-between p-4 rounded-2xl" intensity="md">
                <div className="flex items-center gap-3">
                    <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                        <Bot className="h-4 w-4" />
                    </div>
                    <div>
                        <h2 className="text-sm font-semibold">Job {jobId.substring(0, 8)}</h2>
                        <div className="flex items-center gap-1.5">
                            <span className="relative flex h-2 w-2">
                                <span className={`absolute inline-flex h-full w-full rounded-full opacity-75 ${status === 'connected' ? 'animate-ping bg-green-400' : 'bg-gray-400'}`}></span>
                                <span className={`relative inline-flex rounded-full h-2 w-2 ${status === 'connected' ? 'bg-green-500' : 'bg-gray-500'}`}></span>
                            </span>
                            <span className="text-xs text-muted-foreground capitalize">{status}</span>
                        </div>
                    </div>
                </div>

                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    <div className="flex items-center gap-1">
                        <Cpu className="h-3 w-3" />
                        <span>{latestProgress !== null ? `${latestProgress}%` : "--%"}</span>
                    </div>
                    <div className="flex items-center gap-1">
                        <Zap className="h-3 w-3" />
                        <span>{latestStatus}</span>
                    </div>
                </div>
            </GlassPanel>

            {/* Result Panel */}
            {job?.result && (
                <GlassPanel className="p-4 rounded-2xl" intensity="sm">
                    <div className="flex items-center justify-between mb-3">
                        <h3 className="text-sm font-semibold">Result</h3>
                        <a
                            href={job.result}
                            target="_blank"
                            rel="noreferrer"
                            className="text-xs text-primary underline"
                        >
                            Open file
                        </a>
                    </div>
                    {isImageResult && (
                        <img src={job.result} alt="Job result" className="max-h-64 rounded-lg border border-black/10" />
                    )}
                    {isTextResult && (
                        <iframe src={job.result} className="w-full h-40 rounded-lg border border-black/10 bg-white" title="Text result" />
                    )}
                    {!isImageResult && !isTextResult && (
                        <div className="text-xs text-muted-foreground break-all">{job.result}</div>
                    )}
                </GlassPanel>
            )}

            {/* Main Stream Area */}
            <GlassPanel className="flex-1 p-0 rounded-2xl relative overflow-hidden flex flex-col" intensity="sm" border={false}>
                <div className="absolute inset-0 bg-black/90 pointer-events-none" /> {/* Terminal Background */}

                <div ref={scrollRef} className="relative z-10 flex-1 overflow-y-auto p-4 font-mono text-xs space-y-1">
                    {events.length === 0 && (
                        <div className="text-muted-foreground opacity-50 italic">Waiting for events...</div>
                    )}

                    {events.map((e, i) => (
                        <div key={i} className="flex gap-2">
                            <span className="text-muted-foreground opacity-50">
                                {new Date(e.timestamp * 1000).toLocaleTimeString()}
                            </span>
                            {e.type === 'status' && (
                                <span className="text-blue-400 font-bold">[STATUS] {getStatusLabel(e.data)}</span>
                            )}
                            {e.type === 'log' && (
                                <span className="text-gray-300 break-all whitespace-pre-wrap">{e.data}</span>
                            )}
                        </div>
                    ))}
                </div>

                {/* Input Placeholder */}
                <div className="p-4 border-t border-white/10 z-10 bg-black/40 backdrop-blur-md">
                    <div className="h-10 w-full rounded-lg bg-white/5 border border-white/10 flex items-center px-4 text-sm text-muted-foreground">
                        <Terminal className="h-3 w-3 mr-2" />
                        <span className="opacity-50">Input disabled (Read-only stream)</span>
                    </div>
                </div>
            </GlassPanel>
        </div>
    )
}
