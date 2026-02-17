import { useEffect, useState } from "react"
import {
    Bot,
    Search,
    Code,
    Palette,
    Plus,
    Pencil,
    Trash2,
    Shield,
    Wrench,
    X,
    Check,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { usePersonaStore, type Persona } from "@/store/personas"

const iconMap: Record<string, React.ElementType> = {
    bot: Bot,
    search: Search,
    palette: Palette,
    code: Code,
}

const colorMap: Record<string, { bg: string; border: string; text: string }> = {
    blue: { bg: "bg-blue-500/10", border: "border-blue-500/20", text: "text-blue-600" },
    emerald: { bg: "bg-emerald-500/10", border: "border-emerald-500/20", text: "text-emerald-600" },
    violet: { bg: "bg-violet-500/10", border: "border-violet-500/20", text: "text-violet-600" },
    amber: { bg: "bg-amber-500/10", border: "border-amber-500/20", text: "text-amber-600" },
    cyan: { bg: "bg-cyan-500/10", border: "border-cyan-500/20", text: "text-cyan-600" },
    rose: { bg: "bg-rose-500/10", border: "border-rose-500/20", text: "text-rose-600" },
}

function PersonaCard({ persona, onSelect }: { persona: Persona; onSelect: (p: Persona) => void }) {
    const Icon = iconMap[persona.icon] ?? Bot
    const colors = colorMap[persona.color] ?? colorMap.blue
    const { deletePersona, setActivePersona, activePersonaId } = usePersonaStore()
    const isActive = activePersonaId === persona.id

    return (
        <div
            className={cn(
                "group relative rounded-xl border p-4 transition-all cursor-pointer hover:shadow-md",
                isActive
                    ? `${colors.bg} ${colors.border} ring-2 ring-offset-1 ring-${persona.color}-500/30`
                    : "bg-background/60 border-border/50 hover:border-border"
            )}
            onClick={() => setActivePersona(isActive ? null : persona.id)}
        >
            {/* Actions */}
            <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                {!persona.is_builtin && (
                    <>
                        <button
                            onClick={(e) => { e.stopPropagation(); onSelect(persona) }}
                            className="w-6 h-6 rounded-md bg-background/80 border border-border/50 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors"
                            title="Edit"
                        >
                            <Pencil className="w-3 h-3" />
                        </button>
                        <button
                            onClick={(e) => { e.stopPropagation(); deletePersona(persona.id) }}
                            className="w-6 h-6 rounded-md bg-background/80 border border-border/50 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors"
                            title="Delete"
                        >
                            <Trash2 className="w-3 h-3" />
                        </button>
                    </>
                )}
            </div>

            {/* Header */}
            <div className="flex items-center gap-3 mb-3">
                <div className={cn("w-10 h-10 rounded-xl flex items-center justify-center", colors.bg)}>
                    <Icon className={cn("w-5 h-5", colors.text)} />
                </div>
                <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                        <h3 className="font-semibold text-sm truncate">{persona.name}</h3>
                        {persona.is_builtin && (
                            <span title="Built-in">
                                <Shield className="w-3 h-3 text-muted-foreground flex-shrink-0" />
                            </span>
                        )}
                    </div>
                    <p className="text-xs text-muted-foreground truncate">{persona.description}</p>
                </div>
            </div>

            {/* Tools */}
            <div className="flex items-center gap-1.5 flex-wrap">
                <Wrench className="w-3 h-3 text-muted-foreground" />
                {persona.allowed_tools && persona.allowed_tools.length > 0 ? (
                    persona.allowed_tools.map((t) => (
                        <span key={t} className="text-[10px] px-1.5 py-0.5 rounded bg-muted/50 text-muted-foreground">
                            {t}
                        </span>
                    ))
                ) : (
                    <span className="text-[10px] text-muted-foreground">All tools</span>
                )}
            </div>

            {/* Active indicator */}
            {isActive && (
                <div className={cn("absolute bottom-2 right-3 text-[10px] font-medium", colors.text)}>
                    Active
                </div>
            )}
        </div>
    )
}

function CreatePersonaForm({ onClose, editingPersona }: { onClose: () => void; editingPersona?: Persona }) {
    const { createPersona, updatePersona } = usePersonaStore()
    const [name, setName] = useState(editingPersona?.name ?? "")
    const [description, setDescription] = useState(editingPersona?.description ?? "")
    const [systemPrompt, setSystemPrompt] = useState(editingPersona?.system_prompt ?? "")
    const [icon, setIcon] = useState(editingPersona?.icon ?? "bot")
    const [color, setColor] = useState(editingPersona?.color ?? "blue")
    const [tools, setTools] = useState(editingPersona?.allowed_tools?.join(", ") ?? "")

    const handleSubmit = async () => {
        if (!name.trim() || !systemPrompt.trim()) return
        const allowed = tools.trim() ? tools.split(",").map((t) => t.trim()).filter(Boolean) : undefined

        if (editingPersona) {
            await updatePersona(editingPersona.id, {
                name: name.trim(),
                description: description.trim(),
                system_prompt: systemPrompt.trim(),
                icon,
                color,
                allowed_tools: allowed,
            })
        } else {
            await createPersona({
                name: name.trim(),
                description: description.trim(),
                system_prompt: systemPrompt.trim(),
                icon,
                color,
                allowed_tools: allowed,
            })
        }
        onClose()
    }

    const colors = ["blue", "emerald", "violet", "amber", "cyan", "rose"]
    const icons = ["bot", "search", "palette", "code"]

    return (
        <div className="rounded-xl border border-border/80 bg-background/80 backdrop-blur p-4 space-y-3">
            <div className="flex items-center justify-between">
                <h3 className="font-semibold text-sm">{editingPersona ? "Edit Persona" : "New Persona"}</h3>
                <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
                    <X className="w-4 h-4" />
                </button>
            </div>

            <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name"
                className="w-full bg-muted/50 border border-border/50 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary/30"
            />

            <input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Short description"
                className="w-full bg-muted/50 border border-border/50 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary/30"
            />

            <textarea
                value={systemPrompt}
                onChange={(e) => setSystemPrompt(e.target.value)}
                placeholder="System prompt (instructions for the agent)"
                rows={4}
                className="w-full bg-muted/50 border border-border/50 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary/30 resize-none"
            />

            <input
                value={tools}
                onChange={(e) => setTools(e.target.value)}
                placeholder="Allowed tools (comma-separated, empty = all)"
                className="w-full bg-muted/50 border border-border/50 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary/30"
            />

            {/* Icon / Color selectors */}
            <div className="flex items-center gap-4">
                <div className="flex items-center gap-1">
                    <span className="text-xs text-muted-foreground mr-1">Icon:</span>
                    {icons.map((i) => {
                        const Ic = iconMap[i] ?? Bot
                        return (
                            <button
                                key={i}
                                onClick={() => setIcon(i)}
                                className={cn(
                                    "w-7 h-7 rounded-md flex items-center justify-center transition-colors",
                                    icon === i ? "bg-primary/10 text-primary" : "text-muted-foreground hover:bg-muted"
                                )}
                            >
                                <Ic className="w-3.5 h-3.5" />
                            </button>
                        )
                    })}
                </div>
                <div className="flex items-center gap-1">
                    <span className="text-xs text-muted-foreground mr-1">Color:</span>
                    {colors.map((c) => (
                        <button
                            key={c}
                            onClick={() => setColor(c)}
                            className={cn(
                                "w-5 h-5 rounded-full transition-all",
                                colorMap[c]?.bg ?? "bg-muted",
                                color === c ? "ring-2 ring-offset-1 ring-primary/50 scale-110" : ""
                            )}
                        />
                    ))}
                </div>
            </div>

            <button
                onClick={handleSubmit}
                disabled={!name.trim() || !systemPrompt.trim()}
                className="w-full flex items-center justify-center gap-2 bg-primary text-primary-foreground rounded-lg py-2 text-sm font-medium disabled:opacity-50 hover:bg-primary/90 transition-colors"
            >
                <Check className="w-4 h-4" />
                {editingPersona ? "Save" : "Create"}
            </button>
        </div>
    )
}

export function AgentsView() {
    const { personas, fetchPersonas } = usePersonaStore()
    const [showForm, setShowForm] = useState(false)
    const [editingPersona, setEditingPersona] = useState<Persona | undefined>()

    useEffect(() => {
        fetchPersonas()
    }, [fetchPersonas])

    const handleEdit = (p: Persona) => {
        setEditingPersona(p)
        setShowForm(true)
    }

    const handleClose = () => {
        setShowForm(false)
        setEditingPersona(undefined)
        fetchPersonas()
    }

    return (
        <div className="h-full overflow-y-auto p-6 space-y-6">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Bot className="w-5 h-5 text-primary" />
                    <h1 className="text-lg font-bold">Personas</h1>
                    <span className="text-xs text-muted-foreground">
                        {personas.length} persona{personas.length !== 1 ? "s" : ""}
                    </span>
                </div>
                <button
                    onClick={() => { setEditingPersona(undefined); setShowForm(true) }}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
                >
                    <Plus className="w-3.5 h-3.5" />
                    New Persona
                </button>
            </div>

            {/* Create/Edit form */}
            {showForm && (
                <CreatePersonaForm onClose={handleClose} editingPersona={editingPersona} />
            )}

            {/* Persona grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                {personas.map((p) => (
                    <PersonaCard key={p.id} persona={p} onSelect={handleEdit} />
                ))}
            </div>

            {personas.length === 0 && !showForm && (
                <div className="border border-dashed border-border/80 rounded-xl p-12 text-center text-muted-foreground">
                    <Bot className="w-12 h-12 mx-auto mb-4 opacity-30" />
                    <p className="text-sm font-medium">No personas yet</p>
                    <p className="text-xs mt-1">Personas will appear once the kernel starts and seeds built-in defaults.</p>
                </div>
            )}
        </div>
    )
}
