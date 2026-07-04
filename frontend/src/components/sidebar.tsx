import { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'
import {
  Grid3X3,
  FileText,
  Image,
  Globe,
  Package,
  Folder,
  Settings,
  RefreshCw,
  Plus,
} from 'lucide-react'

const categories = [
  { id: 'all', label: 'sidebar.categories.all', icon: Grid3X3 },
  { id: 'text', label: 'sidebar.categories.text', icon: FileText },
  { id: 'image', label: 'sidebar.categories.image', icon: Image },
  { id: 'network', label: 'sidebar.categories.network', icon: Globe },
  { id: 'dev', label: 'sidebar.categories.dev', icon: Package },
  { id: 'file', label: 'sidebar.categories.file', icon: Folder },
] as const

interface SidebarProps {
  active: string
  onSelectCategory: (id: string) => void
  onAddTool: () => void
  onSettings: () => void
  onRefresh: () => void
  expanded: boolean
}

export function Sidebar({ active, onSelectCategory, onAddTool, onSettings, onRefresh, expanded }: SidebarProps) {
  const { t } = useI18n()
  const [tooltip, setTooltip] = useState<string | null>(null)

  if (expanded) {
    return (
      <aside className="flex w-56 flex-col border-r border-border bg-background overflow-hidden transition-all duration-200">
        <div className="flex items-center justify-between px-3 pt-4 pb-1">
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            {t('sidebar.title')}
          </span>
          <div className="flex gap-1">
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onRefresh} title={t('sidebar.refresh')}>
              <RefreshCw className="h-3.5 w-3.5" />
            </Button>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onAddTool} title={t('sidebar.add')}>
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
        <div className="flex flex-col gap-0.5 px-3 pb-3 pt-1">
          {categories.map((cat) => (
            <Button
              key={cat.id}
              variant="ghost"
              className={cn(
                'relative justify-start gap-3 px-3 font-normal text-sm',
                active === cat.id
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => onSelectCategory(cat.id)}
            >
              {active === cat.id && (
                <span className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary" />
              )}
              <cat.icon className="h-4 w-4" />
              {t(cat.label)}
            </Button>
          ))}
        </div>

        <div className="mt-auto border-t border-border p-3">
          <Button
            variant="ghost"
            className="justify-start gap-3 px-3 w-full font-normal text-muted-foreground hover:text-foreground"
            onClick={onSettings}
          >
            <Settings className="h-4 w-4" />
            {t('sidebar.settings')}
          </Button>
        </div>
      </aside>
    )
  }

  return (
    <aside className="flex w-12 flex-col border-r border-border bg-background overflow-hidden transition-all duration-200">
      <div className="flex flex-col items-center gap-2 py-3">
        {categories.map((cat) => (
          <div key={cat.id} className="relative">
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                'h-9 w-9',
                active === cat.id
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => onSelectCategory(cat.id)}
              onMouseEnter={() => setTooltip(cat.label)}
              onMouseLeave={() => setTooltip(null)}
            >
              <cat.icon className="h-4 w-4" />
            </Button>
            {tooltip === cat.label && (
              <div className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1 text-xs text-popover-foreground shadow-sm">
                {t(cat.label)}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="mt-auto border-t border-border py-3 flex justify-center">
        <div className="relative">
          <Button
            variant="ghost"
            size="icon"
            className="h-9 w-9 text-muted-foreground hover:text-foreground"
            onClick={onSettings}
            onMouseEnter={() => setTooltip(t('sidebar.settings'))}
            onMouseLeave={() => setTooltip(null)}
          >
            <Settings className="h-4 w-4" />
          </Button>
          {tooltip === t('sidebar.settings') && (
            <div className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1 text-xs text-popover-foreground shadow-sm">
              {tooltip}
            </div>
          )}
        </div>
      </div>
    </aside>
  )
}
