import { Cpu, HardDrive, Wifi } from "lucide-react"

export function BottomBar() {
    return (
        <footer className="h-7 flex items-center justify-between px-4 text-[11px] text-muted-foreground bg-background/60 backdrop-blur-xl border-t border-border/50 select-none">
            {/* Left */}
            <div className="flex items-center gap-3">
                <span className="flex items-center gap-1">
                    <Wifi className="w-3 h-3 text-green-500" />
                    <span>Kernel Connected</span>
                </span>
                <span className="flex items-center gap-1">
                    <Cpu className="w-3 h-3" />
                    <span>Ollama</span>
                </span>
            </div>

            {/* Center */}
            <div className="flex items-center gap-2">
                <kbd className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono">⌘K</kbd>
                <span>Search</span>
                <span className="text-border">·</span>
                <kbd className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono">⌘J</kbd>
                <span>Chat</span>
            </div>

            {/* Right */}
            <div className="flex items-center gap-3">
                <span className="flex items-center gap-1">
                    <HardDrive className="w-3 h-3" />
                    <span>Local</span>
                </span>
                <span>auleOS v0.7</span>
            </div>
        </footer>
    )
}
