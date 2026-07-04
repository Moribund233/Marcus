import { Code, FileCode, Clock, Scan, ImageDown, Table, Globe, Terminal, File, Pin, PinOff } from 'lucide-react'
import { cn } from '@/lib/utils'
import { model } from '../../../wailsjs/go/models'

const ICON_MAP: Record<string, React.ReactNode> = {
  Code: <Code className="h-5 w-5" />,
  FileCode: <FileCode className="h-5 w-5" />,
  Clock: <Clock className="h-5 w-5" />,
  Scan: <Scan className="h-5 w-5" />,
  ImageDown: <ImageDown className="h-5 w-5" />,
  Table: <Table className="h-5 w-5" />,
  Globe: <Globe className="h-5 w-5" />,
  Terminal: <Terminal className="h-5 w-5" />,
  File: <File className="h-5 w-5" />,
}

function resolveIcon(icon: string | undefined): React.ReactNode {
  if (!icon) return <Terminal className="h-5 w-5" />
  return ICON_MAP[icon] ?? <Terminal className="h-5 w-5" />
}

interface ToolCardProps {
  tool: model.ToolInfo
  index: number
  visible: boolean
  onClick: () => void
  isPinned?: boolean
  onTogglePin?: (id: string) => void
}

export function ToolCard({ tool, index, visible, onClick, isPinned, onTogglePin }: ToolCardProps) {
  return (
    <div
      onClick={onClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter') onClick() }}
      className="group relative flex flex-col items-start gap-3 overflow-hidden rounded-xl border border-border bg-card p-5 text-left transition-all duration-200 hover:border-primary/30 hover:shadow-[0_0_24px_-4px_hsl(var(--primary)/0.15)] cursor-pointer"
      style={{
        opacity: visible ? 1 : 0,
        transform: visible ? 'translateY(0)' : 'translateY(12px)',
        transition: `opacity 0.4s ease-out ${index * 0.08}s, transform 0.4s ease-out ${index * 0.08}s`,
      }}
    >
      <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
        {resolveIcon(tool.icon)}
      </div>
      <div className="flex-1">
        <div className="text-sm font-medium">{tool.display_name}</div>
        {tool.description && (
          <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground line-clamp-2">
            {tool.description}
          </div>
        )}
      </div>
      {onTogglePin && (
        <button
          onClick={(e) => { e.stopPropagation(); onTogglePin(tool.id) }}
          className={cn(
            'absolute top-3 right-3 opacity-0 transition-all duration-150 group-hover:opacity-100 hover:scale-110',
            isPinned && 'opacity-100',
          )}
        >
          {isPinned
            ? <Pin className="h-3.5 w-3.5 fill-primary text-primary" />
            : <PinOff className="h-3.5 w-3.5 text-muted-foreground/50 hover:text-muted-foreground" />
          }
        </button>
      )}
      <div className="mt-auto flex items-center gap-2 text-[10px] uppercase tracking-wider text-muted-foreground/60">
        <span>{tool.source}</span>
        <span>·</span>
        <span>{tool.contribution}</span>
      </div>
    </div>
  )
}
