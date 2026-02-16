import { GlassPanel } from "@/components/ui/glass-panel"
import { Button } from "@/components/ui/button"
import { api } from "@/lib/api"
import { useState, useEffect, useCallback } from "react"
import { Settings, Cpu, Image, Eye, EyeOff, Plug, Save, RotateCcw, CheckCircle, XCircle, Loader2 } from "lucide-react"

interface SettingsPanelProps {
    onClose?: () => void
}

interface ProviderConfig {
    mode: string
    local_url: string
    remote_url: string
    api_key: string
    default_model: string
}

interface AppConfig {
    providers: {
        llm: ProviderConfig
        image: ProviderConfig
    }
}

interface TestResult {
    status: "ok" | "error" | "testing" | null
    message: string
}

const defaultConfig: AppConfig = {
    providers: {
        llm: { mode: "local", local_url: "http://localhost:11434/v1", remote_url: "", api_key: "", default_model: "gemma3:12b" },
        image: { mode: "local", local_url: "http://localhost:8188", remote_url: "", api_key: "", default_model: "sd-1.5" },
    },
}

const LLM_MODELS = {
    local: ["gemma3:12b", "gemma3:4b", "llama3.2:3b", "llama3.1:8b", "mistral:7b", "qwen2.5:7b", "deepseek-r1:8b"],
    remote: ["gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo", "claude-sonnet-4-20250514", "claude-3-5-haiku-20241022"],
}

const IMAGE_MODELS = {
    local: ["sd-1.5", "sdxl-turbo", "flux-schnell"],
    remote: ["dall-e-3", "dall-e-2", "stable-diffusion-xl"],
}

export function SettingsPanel({ onClose }: SettingsPanelProps) {
    const [config, setConfig] = useState<AppConfig>(defaultConfig)
    const [original, setOriginal] = useState<AppConfig>(defaultConfig)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [showLlmKey, setShowLlmKey] = useState(false)
    const [showImageKey, setShowImageKey] = useState(false)
    const [llmTest, setLlmTest] = useState<TestResult>({ status: null, message: "" })
    const [imageTest, setImageTest] = useState<TestResult>({ status: null, message: "" })
    const [saveSuccess, setSaveSuccess] = useState(false)

    // Load settings from backend
    const loadSettings = useCallback(async () => {
        try {
            const { data, error } = await api.GET("/v1/settings")
            if (error) throw error
            if (data) {
                const loaded: AppConfig = {
                    providers: {
                        llm: {
                            mode: data.providers?.llm?.mode ?? "local",
                            local_url: data.providers?.llm?.local_url ?? "",
                            remote_url: data.providers?.llm?.remote_url ?? "",
                            api_key: data.providers?.llm?.api_key ?? "",
                            default_model: data.providers?.llm?.default_model ?? "",
                        },
                        image: {
                            mode: data.providers?.image?.mode ?? "local",
                            local_url: data.providers?.image?.local_url ?? "",
                            remote_url: data.providers?.image?.remote_url ?? "",
                            api_key: data.providers?.image?.api_key ?? "",
                            default_model: data.providers?.image?.default_model ?? "",
                        },
                    },
                }
                setConfig(loaded)
                setOriginal(loaded)
            }
        } catch (err) {
            console.error("Failed to load settings:", err)
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => { loadSettings() }, [loadSettings])

    const hasChanges = JSON.stringify(config) !== JSON.stringify(original)

    // Save settings
    const handleSave = async () => {
        setSaving(true)
        setSaveSuccess(false)
        try {
            const { data, error } = await api.PUT("/v1/settings", {
                body: {
                    providers: {
                        llm: config.providers.llm as any,
                        image: config.providers.image as any,
                    },
                },
            })
            if (error) throw error
            if (data) {
                const saved: AppConfig = {
                    providers: {
                        llm: {
                            mode: data.providers?.llm?.mode ?? "local",
                            local_url: data.providers?.llm?.local_url ?? "",
                            remote_url: data.providers?.llm?.remote_url ?? "",
                            api_key: data.providers?.llm?.api_key ?? "",
                            default_model: data.providers?.llm?.default_model ?? "",
                        },
                        image: {
                            mode: data.providers?.image?.mode ?? "local",
                            local_url: data.providers?.image?.local_url ?? "",
                            remote_url: data.providers?.image?.remote_url ?? "",
                            api_key: data.providers?.image?.api_key ?? "",
                            default_model: data.providers?.image?.default_model ?? "",
                        },
                    },
                }
                setConfig(saved)
                setOriginal(saved)
                setSaveSuccess(true)
                setTimeout(() => setSaveSuccess(false), 3000)
            }
        } catch (err) {
            console.error("Failed to save settings:", err)
        } finally {
            setSaving(false)
        }
    }

    // Reset to what's saved
    const handleReset = () => {
        setConfig(JSON.parse(JSON.stringify(original)))
        setLlmTest({ status: null, message: "" })
        setImageTest({ status: null, message: "" })
    }

    // Test connection
    const handleTest = async (provider: "llm" | "image") => {
        const setter = provider === "llm" ? setLlmTest : setImageTest
        setter({ status: "testing", message: "Testando conexão..." })

        try {
            const { data, error } = await api.POST("/v1/settings/test", {
                body: { provider },
            })
            if (error) throw error
            setter({
                status: (data as any)?.status === "ok" ? "ok" : "error",
                message: (data as any)?.message ?? "Unknown",
            })
        } catch (err: any) {
            setter({ status: "error", message: err?.message ?? "Connection failed" })
        }
    }

    // Update nested config
    const updateLLM = (field: keyof ProviderConfig, value: string) => {
        setConfig(prev => ({
            ...prev,
            providers: {
                ...prev.providers,
                llm: { ...prev.providers.llm, [field]: value },
            },
        }))
    }

    const updateImage = (field: keyof ProviderConfig, value: string) => {
        setConfig(prev => ({
            ...prev,
            providers: {
                ...prev.providers,
                image: { ...prev.providers.image, [field]: value },
            },
        }))
    }

    if (loading) {
        return (
            <div className="h-full flex items-center justify-center">
                <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
            </div>
        )
    }

    return (
        <div className="h-full flex flex-col gap-4 overflow-y-auto pr-2">
            {/* Header */}
            <GlassPanel className="p-5 rounded-2xl flex-shrink-0" intensity="md">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <Settings className="w-5 h-5" />
                        <div>
                            <h2 className="text-lg font-semibold">Settings</h2>
                            <p className="text-xs text-muted-foreground">Configuração de providers — API keys criptografadas com AES-256-GCM</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        {hasChanges && (
                            <Button variant="ghost" size="sm" onClick={handleReset}>
                                <RotateCcw className="w-4 h-4 mr-1" />
                                Reset
                            </Button>
                        )}
                        <Button
                            variant={saveSuccess ? "outline" : "default"}
                            size="sm"
                            onClick={handleSave}
                            disabled={!hasChanges || saving}
                        >
                            {saving ? (
                                <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                            ) : saveSuccess ? (
                                <CheckCircle className="w-4 h-4 mr-1 text-emerald-600" />
                            ) : (
                                <Save className="w-4 h-4 mr-1" />
                            )}
                            {saveSuccess ? "Salvo!" : "Salvar"}
                        </Button>
                        <Button variant="ghost" size="sm" onClick={onClose}>Voltar</Button>
                    </div>
                </div>
            </GlassPanel>

            {/* LLM Provider */}
            <GlassPanel className="p-5 rounded-2xl flex-shrink-0" intensity="md">
                <div className="flex items-center gap-2 mb-4">
                    <Cpu className="w-5 h-5 text-blue-600" />
                    <h3 className="font-semibold">LLM Provider</h3>
                    <span className="text-xs px-2 py-0.5 bg-blue-100 text-blue-700 rounded-full">
                        {config.providers.llm.mode === "local" ? "Ollama (Local)" : "API Remota"}
                    </span>
                </div>

                {/* Mode Toggle */}
                <div className="mb-4">
                    <label className="text-sm font-medium text-muted-foreground mb-2 block">Mode</label>
                    <div className="flex gap-2">
                        <button
                            onClick={() => updateLLM("mode", "local")}
                            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-all ${config.providers.llm.mode === "local"
                                ? "bg-blue-600 text-white shadow-sm"
                                : "bg-secondary text-muted-foreground hover:bg-secondary/80"
                                }`}
                        >
                            🏠 Local (Ollama)
                        </button>
                        <button
                            onClick={() => updateLLM("mode", "remote")}
                            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-all ${config.providers.llm.mode === "remote"
                                ? "bg-blue-600 text-white shadow-sm"
                                : "bg-secondary text-muted-foreground hover:bg-secondary/80"
                                }`}
                        >
                            ☁️ Remote (API)
                        </button>
                    </div>
                </div>

                <div className="grid gap-3">
                    {config.providers.llm.mode === "local" ? (
                        <FieldInput
                            label="Ollama URL"
                            value={config.providers.llm.local_url}
                            onChange={v => updateLLM("local_url", v)}
                            placeholder="http://localhost:11434/v1"
                            hint="Endereço do servidor Ollama local"
                        />
                    ) : (
                        <>
                            <FieldInput
                                label="API Base URL"
                                value={config.providers.llm.remote_url}
                                onChange={v => updateLLM("remote_url", v)}
                                placeholder="https://api.openai.com/v1"
                                hint="URL base da API (OpenAI, Groq, Together, etc.)"
                            />
                            <div>
                                <label className="text-sm font-medium text-muted-foreground mb-1 block">API Key</label>
                                <div className="relative">
                                    <input
                                        type={showLlmKey ? "text" : "password"}
                                        value={config.providers.llm.api_key}
                                        onChange={e => updateLLM("api_key", e.target.value)}
                                        placeholder="sk-..."
                                        className="w-full px-3 py-2 pr-10 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => setShowLlmKey(!showLlmKey)}
                                        className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                                    >
                                        {showLlmKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                    </button>
                                </div>
                                <p className="text-xs text-muted-foreground mt-1">Criptografada em repouso — nunca armazenada em plaintext</p>
                            </div>
                        </>
                    )}

                    {/* Model selector */}
                    <div>
                        <label className="text-sm font-medium text-muted-foreground mb-1 block">Modelo Padrão</label>
                        <div className="flex gap-2">
                            <select
                                value={config.providers.llm.default_model}
                                onChange={e => updateLLM("default_model", e.target.value)}
                                className="flex-1 px-3 py-2 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                            >
                                {(config.providers.llm.mode === "local" ? LLM_MODELS.local : LLM_MODELS.remote).map(m => (
                                    <option key={m} value={m}>{m}</option>
                                ))}
                            </select>
                            <input
                                type="text"
                                value={config.providers.llm.default_model}
                                onChange={e => updateLLM("default_model", e.target.value)}
                                placeholder="ou digite custom..."
                                className="flex-1 px-3 py-2 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                            />
                        </div>
                    </div>

                    {/* Test Connection */}
                    <div className="flex items-center gap-3 pt-2">
                        <Button variant="outline" size="sm" onClick={() => handleTest("llm")} disabled={llmTest.status === "testing"}>
                            {llmTest.status === "testing" ? (
                                <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                            ) : (
                                <Plug className="w-4 h-4 mr-1" />
                            )}
                            Testar Conexão
                        </Button>
                        {llmTest.status && llmTest.status !== "testing" && (
                            <span className={`text-xs flex items-center gap-1 ${llmTest.status === "ok" ? "text-emerald-600" : "text-red-600"}`}>
                                {llmTest.status === "ok" ? <CheckCircle className="w-3.5 h-3.5" /> : <XCircle className="w-3.5 h-3.5" />}
                                {llmTest.message}
                            </span>
                        )}
                    </div>
                </div>
            </GlassPanel>

            {/* Image Provider */}
            <GlassPanel className="p-5 rounded-2xl flex-shrink-0" intensity="md">
                <div className="flex items-center gap-2 mb-4">
                    <Image className="w-5 h-5 text-purple-600" />
                    <h3 className="font-semibold">Image Provider</h3>
                    <span className="text-xs px-2 py-0.5 bg-purple-100 text-purple-700 rounded-full">
                        {config.providers.image.mode === "local" ? "ComfyUI (Local)" : "API Remota"}
                    </span>
                </div>

                {/* Mode Toggle */}
                <div className="mb-4">
                    <label className="text-sm font-medium text-muted-foreground mb-2 block">Mode</label>
                    <div className="flex gap-2">
                        <button
                            onClick={() => updateImage("mode", "local")}
                            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-all ${config.providers.image.mode === "local"
                                ? "bg-purple-600 text-white shadow-sm"
                                : "bg-secondary text-muted-foreground hover:bg-secondary/80"
                                }`}
                        >
                            🏠 Local (ComfyUI)
                        </button>
                        <button
                            onClick={() => updateImage("mode", "remote")}
                            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-all ${config.providers.image.mode === "remote"
                                ? "bg-purple-600 text-white shadow-sm"
                                : "bg-secondary text-muted-foreground hover:bg-secondary/80"
                                }`}
                        >
                            ☁️ Remote (API)
                        </button>
                    </div>
                </div>

                <div className="grid gap-3">
                    {config.providers.image.mode === "local" ? (
                        <FieldInput
                            label="ComfyUI URL"
                            value={config.providers.image.local_url}
                            onChange={v => updateImage("local_url", v)}
                            placeholder="http://localhost:8188"
                            hint="Endereço do servidor ComfyUI local"
                        />
                    ) : (
                        <>
                            <FieldInput
                                label="API Base URL"
                                value={config.providers.image.remote_url}
                                onChange={v => updateImage("remote_url", v)}
                                placeholder="https://api.openai.com/v1"
                                hint="URL da API de geração de imagem (OpenAI, Replicate, etc.)"
                            />
                            <div>
                                <label className="text-sm font-medium text-muted-foreground mb-1 block">API Key</label>
                                <div className="relative">
                                    <input
                                        type={showImageKey ? "text" : "password"}
                                        value={config.providers.image.api_key}
                                        onChange={e => updateImage("api_key", e.target.value)}
                                        placeholder="sk-..."
                                        className="w-full px-3 py-2 pr-10 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-purple-500/30"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => setShowImageKey(!showImageKey)}
                                        className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                                    >
                                        {showImageKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                    </button>
                                </div>
                                <p className="text-xs text-muted-foreground mt-1">Criptografada em repouso — nunca armazenada em plaintext</p>
                            </div>
                        </>
                    )}

                    {/* Model selector */}
                    <div>
                        <label className="text-sm font-medium text-muted-foreground mb-1 block">Modelo Padrão</label>
                        <div className="flex gap-2">
                            <select
                                value={config.providers.image.default_model}
                                onChange={e => updateImage("default_model", e.target.value)}
                                className="flex-1 px-3 py-2 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-purple-500/30"
                            >
                                {(config.providers.image.mode === "local" ? IMAGE_MODELS.local : IMAGE_MODELS.remote).map(m => (
                                    <option key={m} value={m}>{m}</option>
                                ))}
                            </select>
                            <input
                                type="text"
                                value={config.providers.image.default_model}
                                onChange={e => updateImage("default_model", e.target.value)}
                                placeholder="ou digite custom..."
                                className="flex-1 px-3 py-2 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-purple-500/30"
                            />
                        </div>
                    </div>

                    {/* Test Connection */}
                    <div className="flex items-center gap-3 pt-2">
                        <Button variant="outline" size="sm" onClick={() => handleTest("image")} disabled={imageTest.status === "testing"}>
                            {imageTest.status === "testing" ? (
                                <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                            ) : (
                                <Plug className="w-4 h-4 mr-1" />
                            )}
                            Testar Conexão
                        </Button>
                        {imageTest.status && imageTest.status !== "testing" && (
                            <span className={`text-xs flex items-center gap-1 ${imageTest.status === "ok" ? "text-emerald-600" : "text-red-600"}`}>
                                {imageTest.status === "ok" ? <CheckCircle className="w-3.5 h-3.5" /> : <XCircle className="w-3.5 h-3.5" />}
                                {imageTest.message}
                            </span>
                        )}
                    </div>
                </div>
            </GlassPanel>

            {/* Security Info */}
            <GlassPanel className="p-5 rounded-2xl flex-shrink-0" intensity="md">
                <div className="text-xs text-muted-foreground space-y-2">
                    <p className="font-medium text-foreground text-sm">🔒 Segurança</p>
                    <ul className="space-y-1 list-disc pl-4">
                        <li>API keys são criptografadas com AES-256-GCM antes de serem armazenadas</li>
                        <li>Chave mestra derivada de <code className="bg-secondary px-1 rounded">AULE_SECRET_KEY</code> ou auto-gerada em <code className="bg-secondary px-1 rounded">~/.aule/secret.key</code></li>
                        <li>Keys nunca são retornadas em plaintext pela API — apenas os últimos 4 caracteres são visíveis</li>
                        <li>Ao salvar, providers são recarregados automaticamente sem restart (hot-reload)</li>
                    </ul>
                </div>
            </GlassPanel>
        </div>
    )
}

// Reusable field input component
function FieldInput({ label, value, onChange, placeholder, hint }: {
    label: string
    value: string
    onChange: (v: string) => void
    placeholder: string
    hint?: string
}) {
    return (
        <div>
            <label className="text-sm font-medium text-muted-foreground mb-1 block">{label}</label>
            <input
                type="text"
                value={value}
                onChange={e => onChange(e.target.value)}
                placeholder={placeholder}
                className="w-full px-3 py-2 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30"
            />
            {hint && <p className="text-xs text-muted-foreground mt-1">{hint}</p>}
        </div>
    )
}
