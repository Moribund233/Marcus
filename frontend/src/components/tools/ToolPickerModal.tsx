import { useEffect, useState } from 'react'
import { X, Play, Pin, PinOff, Terminal, Code, FileCode, Clock, Scan, ImageDown, Table, Globe, File } from 'lucide-react'
import { cn } from '@/lib/utils'
import { model } from '../../../wailsjs/go/models'
import { useI18n } from '@/hooks/useI18n'

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

interface ToolPickerModalProps {
  open: boolean
  tools: model.ToolInfo[]
  pinnedIds: string[]
  onClose: () => void
  onSelect: (tool: model.ToolInfo) => void
  onTogglePin: (id: string) => void
}

export function ToolPickerModal({ open, tools, pinnedIds, onClose, onSelect, onTogglePin }: ToolPickerModalProps) {
  const { t } = useI18n()
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (open) {
      requestAnimationFrame(() => setVisible(true))
    } else {
      setVisible(false)
    }
  }, [open])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center"
      onClick={onClose}
    >
      <div
        className={cn(
          'absolute inset-0 bg-black/50 backdrop-blur-sm transition-opacity duration-200',
          visible ? 'opacity-100' : 'opacity-0',
        )}
      />
      <div
        onClick={(e) => e.stopPropagation()}
        className={cn(
          'relative flex max-h-[75vh] w-full max-w-xl flex-col rounded-2xl border border-border bg-background shadow-2xl transition-all duration-200',
          visible ? 'scale-100 opacity-100' : 'scale-95 opacity-0',
        )}
      >
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold">{t('toolPicker.title')}</h2>
          <button
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="overflow-y-auto p-5">
          {tools.length === 0 ? (
            <div className="flex items-center justify-center py-12">
              <p className="text-sm text-muted-foreground/60">{t('toolPicker.empty')}</p>
            </div>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
              {tools.map((tool) => {
                const isPinned = pinnedIds.includes(tool.id)
                return (
                  <div
                    key={tool.id}
                    className="group relative flex flex-col items-center gap-2 rounded-xl border border-border bg-card p-4 transition-all hover:border-primary/30 hover:shadow-[0_0_16px_-4px_hsl(var(--primary)/0.12)]"
                  >
                    <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      {resolveIcon(tool.icon)}
                    </div>
                    <span className="text-xs font-medium text-center line-clamp-2 leading-tight">
                      {tool.display_name}
                    </span>
                    <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                      <button
                        onClick={() => onSelect(tool)}
                        className="flex h-7 w-7 items-center justify-center rounded-md bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground transition-colors"
                        title={t('toolPicker.run')}
                      >
                        <Play className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={(e) => { e.stopPropagation(); onTogglePin(tool.id) }}
                        className={cn(
                          'flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-accent-foreground',
                          isPinned ? 'text-primary' : 'text-muted-foreground',
                        )}
                        title={isPinned ? t('toolPicker.unpin') : t('toolPicker.pin')}
                      >
                        {isPinned ? <Pin className="h-3.5 w-3.5 fill-primary" /> : <PinOff className="h-3.5 w-3.5" />}
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
