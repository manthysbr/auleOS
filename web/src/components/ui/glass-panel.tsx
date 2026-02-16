import * as React from "react"
import { cn } from "@/lib/utils"

interface GlassPanelProps extends React.HTMLAttributes<HTMLDivElement> {
    intensity?: "sm" | "md" | "lg" | "xl"
    border?: boolean
}

const GlassPanel = React.forwardRef<HTMLDivElement, GlassPanelProps>(
    ({ className, intensity = "md", border = true, children, ...props }, ref) => {
        return (
            <div
                ref={ref}
                className={cn(
                    "bg-background/80 backdrop-blur-xl", // Base glass
                    border && "border border-border/50",
                    className
                )}
                {...props}
            >
                {children}
            </div>
        )
    }
)
GlassPanel.displayName = "GlassPanel"

export { GlassPanel }
