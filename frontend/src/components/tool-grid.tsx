import { useEffect, useState } from 'react'
import { ToolCard } from '@/components/tools/ToolCard'
import { model } from '../../wailsjs/go/models'
import { useI18n } from '@/hooks/useI18n'

interface ToolGridProps {
  category: string
  tools: model.ToolInfo[]
  loading: boolean
  onSelectTool: (tool: model.ToolInfo) => void
  onTogglePin?: (id: string) => void
  isPinned?: (id: string) => boolean
}

export function ToolGrid({ category, tools, loading, onSelectTool, onTogglePin, isPinned }: ToolGridProps) {
  const { t } = useI18n()
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    setVisible(false)
    const timer = setTimeout(() => setVisible(true), 60)
    return () => clearTimeout(timer)
  }, [category])

  return (
    <div className="relative flex-1 overflow-y-auto">
      <div className="relative z-10 p-6">
        <div className="mb-8">
          <h1 className="text-2xl font-semibold tracking-tight">{t('toolGrid.title')}</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {tools.length > 0
              ? t('toolGrid.count', { n: tools.length })
              : t('toolGrid.discovering')
            }
          </p>
        </div>

        {loading && tools.length === 0 ? (
          <div className="flex items-center justify-center py-20">
            <p className="text-sm text-muted-foreground/60">{t('toolGrid.scanning')}</p>
          </div>
        ) : tools.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-20">
            <p className="text-sm text-muted-foreground/60">{t('toolGrid.empty')}</p>
            <p className="text-xs text-muted-foreground/40">{t('toolGrid.emptyHint')}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {tools.map((tool, i) => (
              <ToolCard
                key={tool.id}
                tool={tool}
                index={i}
                visible={visible}
                onClick={() => onSelectTool(tool)}
                isPinned={isPinned?.(tool.id)}
                onTogglePin={onTogglePin}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
