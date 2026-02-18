import { useState, useEffect, useCallback } from "react"
import { CalendarClock, Plus, Trash2, Play, Pause, Clock, RefreshCw, Zap, ChevronDown, ChevronUp } from "lucide-react"
import { cn } from "@/lib/utils"

interface ScheduledTask {
    id: string
    name: string
    prompt: string
    command?: string
    type: "one_shot" | "recurring" | "cron"
    cron_expr?: string
    interval_sec?: number
    next_run: string
    last_run?: string
    last_result?: string
    run_count: number
    status: "active" | "paused" | "completed" | "failed"
    created_at: string
    created_by?: string
    deliver?: boolean
}

function formatNextRun(dateStr: string): string {
    const d = new Date(dateStr)
    const now = new Date()
    const diff = d.getTime() - now.getTime()
    if (diff < 0) return "overdue"
    const sec = Math.floor(diff / 1000)
    if (sec < 60) return `in ${sec}s`
    const min = Math.floor(sec / 60)
    if (min < 60) return `in ${min}m`
    const hr = Math.floor(min / 60)
    if (hr < 24) return `in ${hr}h`
    const days = Math.floor(hr / 24)
    return `in ${days}d`
}

function TaskTypeBadge({ type, cronExpr, intervalSec }: { type: ScheduledTask["type"], cronExpr?: string, intervalSec?: number }) {
    if (type === "cron") {
        return (
            <span className="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-purple-500/15 text-purple-400 border border-purple-500/20">
                <RefreshCw className="w-2.5 h-2.5" />
                {cronExpr || "cron"}
            </span>
        )
    }
    if (type === "recurring") {
        const sec = intervalSec ?? 0
        const label = sec < 60 ? `${sec}s` : sec < 3600 ? `${Math.floor(sec / 60)}m` : `${Math.floor(sec / 3600)}h`
        return (
            <span className="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-blue-500/15 text-blue-400 border border-blue-500/20">
                <RefreshCw className="w-2.5 h-2.5" />
                every {label}
            </span>
        )
    }
    return (
        <span className="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] bg-amber-500/15 text-amber-400 border border-amber-500/20">
            <Zap className="w-2.5 h-2.5" />
            one-shot
        </span>
    )
}

function StatusDot({ status }: { status: ScheduledTask["status"] }) {
    const colors: Record<ScheduledTask["status"], string> = {
        active: "bg-emerald-400 shadow-emerald-400/50",
        paused: "bg-amber-400 shadow-amber-400/50",
        completed: "bg-blue-400 shadow-blue-400/50",
        failed: "bg-red-400 shadow-red-400/50",
    }
    return (
        <span className={cn("w-2 h-2 rounded-full flex-shrink-0 shadow-sm", colors[status])} />
    )
}

interface CreateTaskForm {
    name: string
    prompt: string
    type: ScheduledTask["type"]
    cron_expr: string
    interval_sec: number
}

const EMPTY_FORM: CreateTaskForm = {
    name: "",
    prompt: "",
    type: "one_shot",
    cron_expr: "",
    interval_sec: 3600,
}

export function ScheduledTasksView() {
    const [tasks, setTasks] = useState<ScheduledTask[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [expandedId, setExpandedId] = useState<string | null>(null)
    const [showCreate, setShowCreate] = useState(false)
    const [form, setForm] = useState<CreateTaskForm>(EMPTY_FORM)
    const [submitting, setSubmitting] = useState(false)

    const fetchTasks = useCallback(async () => {
        try {
            const res = await fetch("/v1/tasks")
            if (!res.ok) throw new Error(await res.text())
            const data = await res.json()
            setTasks(data.tasks ?? [])
            setError(null)
        } catch (e: unknown) {
            setError((e as Error).message)
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchTasks()
        const id = setInterval(fetchTasks, 10_000)
        return () => clearInterval(id)
    }, [fetchTasks])

    const toggle = async (id: string) => {
        await fetch(`/v1/tasks/${id}/toggle`, { method: "POST" })
        fetchTasks()
    }

    const deleteTask = async (id: string) => {
        await fetch(`/v1/tasks/${id}`, { method: "DELETE" })
        fetchTasks()
    }

    const handleCreate = async () => {
        if (!form.name.trim() || !form.prompt.trim()) return
        setSubmitting(true)
        try {
            const body: Record<string, unknown> = {
                name: form.name,
                prompt: form.prompt,
                type: form.type,
                status: "active",
                next_run: new Date().toISOString(),
            }
            if (form.type === "cron") body.cron_expr = form.cron_expr
            if (form.type === "recurring") body.interval_sec = form.interval_sec
            const res = await fetch("/v1/tasks", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(body),
            })
            if (!res.ok) throw new Error(await res.text())
            await fetchTasks()
            setShowCreate(false)
            setForm(EMPTY_FORM)
        } catch (e: unknown) {
            setError((e as Error).message)
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="h-full flex flex-col bg-background">
            {/* Header */}
            <div className="flex-shrink-0 px-6 py-4 border-b border-border/50 flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-lg bg-amber-500/10 flex items-center justify-center">
                        <CalendarClock className="w-4 h-4 text-amber-400" />
                    </div>
                    <div>
                        <h1 className="text-sm font-semibold text-foreground">Scheduled Tasks</h1>
                        <p className="text-xs text-muted-foreground">{tasks.length} task{tasks.length !== 1 ? "s" : ""} scheduled</p>
                    </div>
                </div>
                <button
                    onClick={() => setShowCreate(v => !v)}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
                >
                    <Plus className="w-3 h-3" />
                    New Task
                </button>
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-3">
                {/* Create form */}
                {showCreate && (
                    <div className="rounded-xl border border-border/50 bg-card/50 p-4 space-y-3">
                        <p className="text-xs font-medium text-foreground">New Scheduled Task</p>
                        <div className="grid grid-cols-2 gap-2">
                            <div className="col-span-2">
                                <label className="block text-xs text-muted-foreground mb-1">Name</label>
                                <input
                                    value={form.name}
                                    onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                                    className="w-full text-xs bg-background border border-border/50 rounded-lg px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                                    placeholder="Daily summary"
                                />
                            </div>
                            <div className="col-span-2">
                                <label className="block text-xs text-muted-foreground mb-1">Prompt</label>
                                <textarea
                                    value={form.prompt}
                                    onChange={e => setForm(f => ({ ...f, prompt: e.target.value }))}
                                    rows={2}
                                    className="w-full text-xs bg-background border border-border/50 rounded-lg px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary/50 resize-none"
                                    placeholder="Summarize all tasks completed today..."
                                />
                            </div>
                            <div>
                                <label className="block text-xs text-muted-foreground mb-1">Type</label>
                                <select
                                    value={form.type}
                                    onChange={e => setForm(f => ({ ...f, type: e.target.value as ScheduledTask["type"] }))}
                                    className="w-full text-xs bg-background border border-border/50 rounded-lg px-3 py-2 text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                                >
                                    <option value="one_shot">One-shot</option>
                                    <option value="recurring">Recurring</option>
                                    <option value="cron">Cron</option>
                                </select>
                            </div>
                            {form.type === "cron" && (
                                <div>
                                    <label className="block text-xs text-muted-foreground mb-1">Cron Expression</label>
                                    <input
                                        value={form.cron_expr}
                                        onChange={e => setForm(f => ({ ...f, cron_expr: e.target.value }))}
                                        className="w-full text-xs bg-background border border-border/50 rounded-lg px-3 py-2 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary/50 font-mono"
                                        placeholder="0 9 * * *"
                                    />
                                </div>
                            )}
                            {form.type === "recurring" && (
                                <div>
                                    <label className="block text-xs text-muted-foreground mb-1">Interval (seconds)</label>
                                    <input
                                        type="number"
                                        value={form.interval_sec}
                                        onChange={e => setForm(f => ({ ...f, interval_sec: parseInt(e.target.value) || 3600 }))}
                                        className="w-full text-xs bg-background border border-border/50 rounded-lg px-3 py-2 text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                                        min={60}
                                    />
                                </div>
                            )}
                        </div>
                        <div className="flex justify-end gap-2">
                            <button onClick={() => { setShowCreate(false); setForm(EMPTY_FORM) }} className="px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors">Cancel</button>
                            <button
                                onClick={handleCreate}
                                disabled={submitting || !form.name.trim() || !form.prompt.trim()}
                                className="px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {submitting ? "Creating..." : "Create"}
                            </button>
                        </div>
                    </div>
                )}

                {/* Error */}
                {error && (
                    <div className="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded-lg px-4 py-2">{error}</div>
                )}

                {/* Loading */}
                {loading && !tasks.length && (
                    <div className="flex items-center justify-center py-12 text-muted-foreground text-sm">
                        <RefreshCw className="w-4 h-4 animate-spin mr-2" />
                        Loading tasks...
                    </div>
                )}

                {/* Empty */}
                {!loading && !tasks.length && (
                    <div className="flex flex-col items-center justify-center py-20 gap-3 text-center">
                        <div className="w-12 h-12 rounded-2xl bg-muted/30 flex items-center justify-center">
                            <CalendarClock className="w-6 h-6 text-muted-foreground" />
                        </div>
                        <p className="text-sm font-medium text-muted-foreground">No scheduled tasks yet</p>
                        <p className="text-xs text-muted-foreground/60">Create a task to automate recurring prompts</p>
                        <button
                            onClick={() => setShowCreate(true)}
                            className="mt-2 flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
                        >
                            <Plus className="w-3 h-3" />
                            Create first task
                        </button>
                    </div>
                )}

                {/* Task list */}
                {tasks.map(task => (
                    <div
                        key={task.id}
                        className="rounded-xl border border-border/50 bg-card/30 hover:bg-card/60 transition-all overflow-hidden"
                    >
                        <div className="flex items-center gap-3 px-4 py-3">
                            <StatusDot status={task.status} />
                            <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 flex-wrap">
                                    <span className="text-sm font-medium text-foreground truncate">{task.name}</span>
                                    <TaskTypeBadge type={task.type} cronExpr={task.cron_expr} intervalSec={task.interval_sec} />
                                    {task.status === "paused" && (
                                        <span className="px-2 py-0.5 rounded-full text-[10px] bg-amber-500/15 text-amber-400 border border-amber-500/20">paused</span>
                                    )}
                                </div>
                                <div className="flex items-center gap-3 mt-0.5">
                                    <span className="flex items-center gap-1 text-[10px] text-muted-foreground">
                                        <Clock className="w-2.5 h-2.5" />
                                        {formatNextRun(task.next_run)}
                                    </span>
                                    <span className="text-[10px] text-muted-foreground">
                                        {task.run_count} run{task.run_count !== 1 ? "s" : ""}
                                    </span>
                                </div>
                            </div>

                            {/* Actions */}
                            <div className="flex items-center gap-1 flex-shrink-0">
                                <button
                                    onClick={() => toggle(task.id)}
                                    className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                                    title={task.status === "active" ? "Pause" : "Resume"}
                                >
                                    {task.status === "active" ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
                                </button>
                                <button
                                    onClick={() => deleteTask(task.id)}
                                    className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:text-red-400 hover:bg-red-400/10 transition-colors"
                                    title="Delete"
                                >
                                    <Trash2 className="w-3.5 h-3.5" />
                                </button>
                                <button
                                    onClick={() => setExpandedId(v => v === task.id ? null : task.id)}
                                    className="w-7 h-7 rounded-lg flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                                >
                                    {expandedId === task.id ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                                </button>
                            </div>
                        </div>

                        {/* Expanded detail */}
                        {expandedId === task.id && (
                            <div className="px-4 pt-0 pb-4 border-t border-border/30 space-y-2">
                                <div>
                                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground/60 mb-1">Prompt</p>
                                    <p className="text-xs text-muted-foreground bg-muted/20 rounded-lg px-3 py-2">{task.prompt}</p>
                                </div>
                                {task.last_result && (
                                    <div>
                                        <p className="text-[10px] uppercase tracking-wider text-muted-foreground/60 mb-1">Last result</p>
                                        <p className="text-xs text-muted-foreground bg-muted/20 rounded-lg px-3 py-2 line-clamp-3">{task.last_result}</p>
                                    </div>
                                )}
                                {task.last_run && (
                                    <p className="text-[10px] text-muted-foreground/50">
                                        Last ran: {new Date(task.last_run).toLocaleString()}
                                    </p>
                                )}
                                {task.created_by && (
                                    <p className="text-[10px] text-muted-foreground/50">
                                        Created by: {task.created_by}
                                    </p>
                                )}
                            </div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    )
}
