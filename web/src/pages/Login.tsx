import { useState } from 'react'
import { useLocation } from 'wouter'
import { motion } from 'framer-motion'
import { Button } from '@/components/ui/button'
import { GlassPanel } from '@/components/ui/glass-panel'
import { ArrowRight, Terminal } from 'lucide-react'

export default function Login() {
    const [_, setLocation] = useLocation()
    const [loading, setLoading] = useState(false)

    const handleConnect = () => {
        setLoading(true)
        // Simulate connection
        setTimeout(() => {
            setLocation('/workspace')
        }, 1500)
    }

    return (
        <div className="flex min-h-screen w-full items-center justify-center bg-background dot-pattern relative overflow-hidden">
            {/* Background Ambience */}
            <div className="absolute inset-0 bg-gradient-to-tr from-primary/5 via-transparent to-transparent pointer-events-none" />

            <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5, ease: "easeOut" }}
                className="w-full max-w-sm"
            >
                <GlassPanel className="p-8 flex flex-col gap-6" intensity="xl">
                    <div className="flex flex-col items-center gap-2 text-center">
                        <div className="h-12 w-12 rounded-xl bg-primary/10 flex items-center justify-center text-primary mb-2">
                            <Terminal className="h-6 w-6" />
                        </div>
                        <h1 className="text-2xl font-semibold tracking-tight">auleOS Kernel</h1>
                        <p className="text-sm text-muted-foreground">
                            Secure Agentic Environment
                        </p>
                    </div>

                    <div className="grid gap-4">
                        <Button
                            onClick={handleConnect}
                            disabled={loading}
                            className="w-full font-medium"
                            size="lg"
                        >
                            {loading ? (
                                <span className="flex items-center gap-2">
                                    <span className="h-4 w-4 rounded-full border-2 border-current border-t-transparent animate-spin" />
                                    Connecting...
                                </span>
                            ) : (
                                <span className="flex items-center gap-2">
                                    Connect to Localhost <ArrowRight className="h-4 w-4" />
                                </span>
                            )}
                        </Button>
                    </div>

                    <div className="text-center text-xs text-muted-foreground/50">
                        v0.1.0 • Stable Channel
                    </div>
                </GlassPanel>
            </motion.div>
        </div>
    )
}
