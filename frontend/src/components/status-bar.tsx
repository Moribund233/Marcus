import { CheckCircle2, AlertCircle, Minus } from 'lucide-react'
import type { RuntimeInfo } from '@/components/renderer/types'

interface StatusBarProps {
  message: string
  runtimeStatus: Record<string, RuntimeInfo>
}

const RUNTIME_ORDER = ['python', 'node', 'uv', 'bun']

const RUNTIME_LABELS: Record<string, string> = {
  python: 'Python',
  node: 'Node',
  uv: 'uv',
  bun: 'bun',
}

export function StatusBar({ message, runtimeStatus }: StatusBarProps) {
  return (
    <div className="flex h-6 shrink-0 items-center justify-between border-t border-border bg-card px-3 text-[11px] text-muted-foreground/70 select-none">
      <div className="flex items-center gap-2 truncate">
        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
        <span className="truncate">{message}</span>
      </div>

      <div className="flex items-center gap-3">
        {RUNTIME_ORDER.map((key) => {
          const info = runtimeStatus[key]
          if (!info) return null
          return (
            <span key={key} className="flex items-center gap-1">
              {info.available
                ? <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                : <Minus className="h-3 w-3 text-muted-foreground/30" />
              }
              <span className={info.available ? 'text-muted-foreground/70' : 'text-muted-foreground/30'}>
                {RUNTIME_LABELS[key] ?? key}
              </span>
            </span>
          )
        })}
      </div>
    </div>
  )
}
