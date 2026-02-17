import { Wrench, ImageIcon, FileText, Search, Mic, Eye, Code } from "lucide-react"
import { cn } from "@/lib/utils"

const tools = [
    {
        name: "generate_image",
        description: "Generate images via ComfyUI / Stable Diffusion",
        icon: ImageIcon,
        status: "active" as const,
        capability: "image.generate",
    },
    {
        name: "generate_text",
        description: "Generate text via Ollama / OpenAI",
        icon: FileText,
        status: "active" as const,
        capability: "text.generate",
    },
    {
        name: "web_search",
        description: "Search the web via SearXNG",
        icon: Search,
        status: "planned" as const,
        capability: "web.search",
    },
    {
        name: "analyze_image",
        description: "Analyze images with Moondream2 / GPT-4V",
        icon: Eye,
        status: "planned" as const,
        capability: "image.analyze",
    },
    {
        name: "generate_audio",
        description: "Text-to-speech via Piper/Kokoro",
        icon: Mic,
        status: "planned" as const,
        capability: "audio.generate",
    },
    {
        name: "run_code",
        description: "Execute sandboxed code",
        icon: Code,
        status: "planned" as const,
        capability: "code.execute",
    },
]

export function ToolsView() {
    return (
        <div className="h-full overflow-y-auto p-6 space-y-6">
            <div className="flex items-center gap-2">
                <Wrench className="w-5 h-5 text-primary" />
                <h1 className="text-lg font-bold">Tools</h1>
                <span className="text-sm text-muted-foreground">({tools.filter((t) => t.status === "active").length} active)</span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {tools.map((tool) => {
                    const Icon = tool.icon
                    return (
                        <div
                            key={tool.name}
                            className={cn(
                                "p-4 rounded-xl border transition-all",
                                tool.status === "active"
                                    ? "bg-card/60 border-border/50 hover:border-primary/30"
                                    : "bg-muted/20 border-border/30 opacity-60"
                            )}
                        >
                            <div className="flex items-center gap-3 mb-2">
                                <div className={cn(
                                    "w-9 h-9 rounded-lg flex items-center justify-center",
                                    tool.status === "active" ? "bg-primary/10" : "bg-muted/50"
                                )}>
                                    <Icon className={cn("w-4 h-4", tool.status === "active" ? "text-primary" : "text-muted-foreground")} />
                                </div>
                                <div>
                                    <p className="text-sm font-medium font-mono">{tool.name}</p>
                                    <p className="text-[10px] text-muted-foreground">{tool.capability}</p>
                                </div>
                            </div>
                            <p className="text-xs text-muted-foreground">{tool.description}</p>
                            <div className="mt-3">
                                <span className={cn(
                                    "text-[10px] font-medium px-2 py-0.5 rounded-full",
                                    tool.status === "active"
                                        ? "bg-green-500/10 text-green-600"
                                        : "bg-muted text-muted-foreground"
                                )}>
                                    {tool.status === "active" ? "● Active" : "○ Planned"}
                                </span>
                            </div>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}
