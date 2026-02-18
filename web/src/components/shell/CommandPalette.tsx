import { useState, useEffect, useRef, useCallback } from "react"
import { Search, Home, FolderKanban, Bot, Wrench, Activity, Settings, MessageSquare, X } from "lucide-react"
import { cn } from "@/lib/utils"
import { useUIStore } from "@/store/ui"
import { useProjectStore } from "@/store/projects"
import { useConversationStore } from "@/store/conversations"

interface Command {
    id: string
    label: string
    icon: React.ElementType
    category: string
    action: () => void
}

export function CommandPalette() {
    const { commandPaletteOpen, setCommandPaletteOpen, setActiveView, openProject, setChatWindowOpen } = useUIStore()
    const { projects } = useProjectStore()
    const { conversations, selectConversation } = useConversationStore()
    const [query, setQuery] = useState("")
    const inputRef = useRef<HTMLInputElement>(null)

    const commands: Command[] = [
        // Navigation
        { id: "nav-home", label: "Go to Dashboard", icon: Home, category: "Navigation", action: () => setActiveView("dashboard") },
        { id: "nav-projects", label: "Go to Projects", icon: FolderKanban, category: "Navigation", action: () => setActiveView("project") },
        { id: "nav-agents", label: "Go to Agents", icon: Bot, category: "Navigation", action: () => setActiveView("agents") },
        { id: "nav-tools", label: "Go to Tools", icon: Wrench, category: "Navigation", action: () => setActiveView("tools") },
        { id: "nav-jobs", label: "Go to Jobs", icon: Activity, category: "Navigation", action: () => setActiveView("jobs") },
        { id: "nav-settings", label: "Go to Settings", icon: Settings, category: "Navigation", action: () => setActiveView("settings") },
        { id: "toggle-chat", label: "Abrir Chat", icon: MessageSquare, category: "Actions", action: () => setChatWindowOpen(true) },
        // Projects
        ...projects.map((p) => ({
            id: `proj-${p.id}`,
            label: `Open: ${p.name}`,
            icon: FolderKanban,
            category: "Projects",
            action: () => openProject(p.id),
        })),
        // Conversations
        ...conversations.slice(0, 10).map((c) => ({
            id: `conv-${c.id}`,
            label: `Chat: ${c.title || "Untitled"}`,
            icon: MessageSquare,
            category: "Conversations",
            action: () => {
                selectConversation(c.id)
                setChatWindowOpen(true)
            },
        })),
    ]

    const filtered = query.trim()
        ? commands.filter((cmd) => cmd.label.toLowerCase().includes(query.toLowerCase()))
        : commands

    const [selectedIndex, setSelectedIndex] = useState(0)

    // Reset on open
    useEffect(() => {
        if (commandPaletteOpen) {
            setQuery("")
            setSelectedIndex(0)
            setTimeout(() => inputRef.current?.focus(), 50)
        }
    }, [commandPaletteOpen])

    // Reset selection when filter changes
    useEffect(() => {
        setSelectedIndex(0)
    }, [query])

    // Keyboard shortcut
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "k") {
                e.preventDefault()
                setCommandPaletteOpen(!commandPaletteOpen)
            }
            if ((e.metaKey || e.ctrlKey) && e.key === "j") {
                e.preventDefault()
                useUIStore.getState().toggleChatWindow()
            }
        }
        window.addEventListener("keydown", handleKeyDown)
        return () => window.removeEventListener("keydown", handleKeyDown)
    }, [commandPaletteOpen, setCommandPaletteOpen])

    const execute = useCallback((cmd: Command) => {
        cmd.action()
        setCommandPaletteOpen(false)
    }, [setCommandPaletteOpen])

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "ArrowDown") {
            e.preventDefault()
            setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1))
        } else if (e.key === "ArrowUp") {
            e.preventDefault()
            setSelectedIndex((i) => Math.max(i - 1, 0))
        } else if (e.key === "Enter" && filtered[selectedIndex]) {
            e.preventDefault()
            execute(filtered[selectedIndex])
        } else if (e.key === "Escape") {
            setCommandPaletteOpen(false)
        }
    }

    if (!commandPaletteOpen) return null

    // Group by category
    const grouped = new Map<string, Command[]>()
    for (const cmd of filtered) {
        const arr = grouped.get(cmd.category) ?? []
        arr.push(cmd)
        grouped.set(cmd.category, arr)
    }

    let flatIndex = 0

    return (
        <div className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh]">
            {/* Backdrop */}
            <div
                className="absolute inset-0 bg-black/40 backdrop-blur-sm"
                onClick={() => setCommandPaletteOpen(false)}
            />

            {/* Palette */}
            <div className="relative w-full max-w-lg bg-popover border border-border shadow-2xl rounded-xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
                {/* Input */}
                <div className="flex items-center gap-2 px-4 border-b border-border">
                    <Search className="w-4 h-4 text-muted-foreground flex-shrink-0" />
                    <input
                        ref={inputRef}
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="Search commands, projects, conversations..."
                        className="flex-1 h-11 bg-transparent text-sm focus:outline-none placeholder:text-muted-foreground/60"
                    />
                    <button
                        onClick={() => setCommandPaletteOpen(false)}
                        className="text-muted-foreground hover:text-foreground"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                {/* Results */}
                <div className="max-h-72 overflow-y-auto p-1">
                    {filtered.length === 0 && (
                        <div className="p-6 text-center text-sm text-muted-foreground">
                            No results found
                        </div>
                    )}
                    {Array.from(grouped.entries()).map(([category, cmds]) => (
                        <div key={category}>
                            <p className="px-3 py-1.5 text-[10px] font-semibold uppercase text-muted-foreground/60 tracking-wider">
                                {category}
                            </p>
                            {cmds.map((cmd) => {
                                const idx = flatIndex++
                                const Icon = cmd.icon
                                return (
                                    <button
                                        key={cmd.id}
                                        onClick={() => execute(cmd)}
                                        onMouseEnter={() => setSelectedIndex(idx)}
                                        className={cn(
                                            "w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-left transition-colors",
                                            idx === selectedIndex
                                                ? "bg-accent text-accent-foreground"
                                                : "text-foreground hover:bg-accent/50"
                                        )}
                                    >
                                        <Icon className="w-4 h-4 text-muted-foreground flex-shrink-0" />
                                        <span className="flex-1 truncate">{cmd.label}</span>
                                        {idx === selectedIndex && (
                                            <kbd className="text-[10px] px-1.5 py-0.5 rounded bg-muted font-mono">↵</kbd>
                                        )}
                                    </button>
                                )
                            })}
                        </div>
                    ))}
                </div>

                {/* Footer */}
                <div className="border-t border-border px-3 py-2 flex items-center gap-3 text-[10px] text-muted-foreground">
                    <span className="flex items-center gap-1">
                        <kbd className="px-1 py-0.5 rounded bg-muted font-mono">↑↓</kbd> navigate
                    </span>
                    <span className="flex items-center gap-1">
                        <kbd className="px-1 py-0.5 rounded bg-muted font-mono">↵</kbd> select
                    </span>
                    <span className="flex items-center gap-1">
                        <kbd className="px-1 py-0.5 rounded bg-muted font-mono">esc</kbd> close
                    </span>
                </div>
            </div>
        </div>
    )
}
