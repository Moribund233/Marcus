import { useState } from 'react'
import { Pin, Play, Plus, Terminal, Globe, File } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useI18n } from '@/hooks/useI18n'
import { model } from '../../wailsjs/go/models'
import type { ToolManifest } from '@/components/renderer/types'

interface RightSidebarProps {
  runningTools: model.ToolInfo[]
  pinnedTools: model.ToolInfo[]
  onSelectTool: (tool: model.ToolInfo) => void
  onTogglePin: (toolId: string) => void
  onShowToolPicker: () => void
}

function resolveContributionIcon(contribution: string) {
  switch (contribution) {
    case 'web': return <Globe className="h-4 w-4" />
    case 'terminal': return <Terminal className="h-4 w-4" />
    case 'file': return <File className="h-4 w-4" />
    default: return <Terminal className="h-4 w-4" />
  }
}

function ToolIcon({ tool }: { tool: model.ToolInfo }) {
  const fallback = resolveContributionIcon(tool.contribution)
  return <>{fallback}</>
}

interface ToolGroupProps {
  label: string
  tools: model.ToolInfo[]
  onSelect: (tool: model.ToolInfo) => void
  onTogglePin?: (id: string) => void
  showPin?: boolean
  isRunning?: boolean
}

function ToolGroup({ label, tools, onSelect, onTogglePin, showPin, isRunning }: ToolGroupProps) {
  const [tooltip, setTooltip] = useState<string | null>(null)

  if (tools.length === 0) return null

  return (
    <div className="flex flex-col items-center gap-1">
      {tools.map((tool) => (
        <div key={tool.id} className="relative">
          <button
            className={cn(
              'flex h-10 w-10 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
              isRunning && 'text-primary',
            )}
            onClick={() => onSelect(tool)}
            onMouseEnter={() => setTooltip(tool.display_name)}
            onMouseLeave={() => setTooltip(null)}
          >
            <div className="relative">
              <ToolIcon tool={tool} />
              {isRunning && (
                <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_6px_0_hsl(160_100%_40%/0.6)]" />
              )}
            </div>
          </button>
          {tooltip && (
            <div className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1 text-xs text-popover-foreground shadow-sm">
              {tooltip}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

export function RightSidebar({ runningTools, pinnedTools, onSelectTool, onTogglePin, onShowToolPicker }: RightSidebarProps) {
  const { t } = useI18n()
  const [tooltip, setTooltip] = useState<string | null>(null)
  const hasTools = runningTools.length > 0 || pinnedTools.length > 0

  return (
    <aside className="flex w-12 flex-col items-center border-l border-border bg-background py-3 gap-4">
      <div className="flex flex-col items-center gap-2">
        <div className="relative">
          <button
            onClick={onShowToolPicker}
            onMouseEnter={() => setTooltip(t('rightSidebar.pickTool'))}
            onMouseLeave={() => setTooltip(null)}
            className={cn(
              'flex h-8 w-8 items-center justify-center rounded-lg transition-all duration-200',
              hasTools
                ? 'text-muted-foreground/40 hover:text-muted-foreground hover:bg-accent'
                : 'text-primary/60 hover:text-primary hover:bg-primary/10 border border-dashed border-border hover:border-primary/30',
            )}
          >
            <Plus className="h-4 w-4" />
          </button>
          {tooltip === t('rightSidebar.pickTool') && (
            <div className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1 text-xs text-popover-foreground shadow-sm">
              {tooltip}
            </div>
          )}
        </div>
      </div>
      <ToolGroup
        label={t('rightSidebar.running')}
        tools={runningTools}
        onSelect={onSelectTool}
        isRunning
      />
      <ToolGroup
        label={t('rightSidebar.pinned')}
        tools={pinnedTools}
        onSelect={onSelectTool}
        showPin
        onTogglePin={onTogglePin}
      />
    </aside>
  )
}
