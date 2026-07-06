import { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'
import { model } from '../../wailsjs/go/models'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  MessageSquare,
  ChevronDown,
  ChevronUp,
  Edit3,
  Trash2,
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
  activeView: 'tools' | 'chat' | 'welcome'
  activeCategory: string
  activeConversationId?: string
  conversations: model.Conversation[]
  expanded: boolean
  onSelectCategory: (id: string) => void
  onSelectConversation: (id: string) => void
  onNewConversation: () => void
  onDeleteConversation?: (id: string) => void
  onAddTool: () => void
  onSettings: () => void
  onRefresh: () => void
}

/**
 * 侧边栏组件，将会话列表与工具分类上下分区展示。
 *
 * 展开模式下：上半部分显示可折叠的会话列表，下半部分显示可折叠的工具分类，
 * 最底部固定设置入口。
 * 折叠模式下：顶部为会话入口，中部为工具分类图标，底部为设置入口。
 */
export function Sidebar({
  activeView,
  activeCategory,
  activeConversationId,
  conversations,
  expanded,
  onSelectCategory,
  onSelectConversation,
  onNewConversation,
  onDeleteConversation,
  onAddTool,
  onSettings,
  onRefresh,
}: SidebarProps) {
  const { t } = useI18n()
  const [tooltip, setTooltip] = useState<string | null>(null)
  const [convExpanded, setConvExpanded] = useState(true)
  const [toolExpanded, setToolExpanded] = useState(true)
  const [deleteTarget, setDeleteTarget] = useState<model.Conversation | null>(null)

  const handleConfirmDelete = () => {
    if (deleteTarget && onDeleteConversation) {
      onDeleteConversation(deleteTarget.id)
    }
    setDeleteTarget(null)
  }

  if (expanded) {
    return (
      <aside className="flex w-56 flex-col border-r border-border bg-background overflow-hidden transition-all duration-200">
        {/* 会话分区 — 占据上半 1/2 */}
        <div className="flex flex-1 flex-col min-h-0">
          <div className="flex items-center justify-between px-3 pt-4 pb-1">
            <button
              className="flex items-center gap-1 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground"
              onClick={() => setConvExpanded((v) => !v)}
            >
              {convExpanded ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />}
              {t('sidebar.conversations.title')}
            </button>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={onNewConversation}
              title={t('sidebar.conversations.new')}
            >
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>

          {convExpanded && (
            <div className="flex flex-col gap-0.5 px-3 pb-2 overflow-y-auto min-h-0">
              {conversations.length === 0 && (
                <div className="px-3 py-2 text-xs text-muted-foreground">
                  {t('sidebar.conversations.empty')}
                </div>
              )}
              {conversations.map((conv) => (
                <div key={conv.id} className="group flex items-center">
                  <Button
                    variant="ghost"
                    className={cn(
                      'flex-1 justify-start gap-2 px-3 font-normal text-sm truncate',
                      activeView === 'chat' && activeConversationId === conv.id
                        ? 'bg-accent text-accent-foreground'
                        : 'text-muted-foreground hover:text-foreground',
                    )}
                    onClick={() => onSelectConversation(conv.id)}
                  >
                    <MessageSquare className="h-3.5 w-3.5 shrink-0" />
                    <span className="truncate">{conv.title}</span>
                  </Button>
                  {onDeleteConversation && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive"
                      onClick={(e) => {
                        e.stopPropagation()
                        setDeleteTarget(conv)
                      }}
                      title={t('sidebar.conversations.delete')}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 工具分类分区 — 占据下半 1/2 */}
        <div className="flex flex-1 flex-col min-h-0 border-t border-border">
          <div className="flex items-center justify-between px-3 pt-3 pb-1">
            <button
              className="flex items-center gap-1 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground"
              onClick={() => setToolExpanded((v) => !v)}
            >
              {toolExpanded ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />}
              {t('sidebar.tools.title')}
            </button>
            <div className="flex gap-1">
              <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onRefresh} title={t('sidebar.refresh')}>
                <RefreshCw className="h-3.5 w-3.5" />
              </Button>
              <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onAddTool} title={t('sidebar.add')}>
                <Edit3 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>

          {toolExpanded && (
            <div className="flex flex-col gap-0.5 px-3 pb-3 pt-1 overflow-y-auto min-h-0">
              {categories.map((cat) => (
                <Button
                  key={cat.id}
                  variant="ghost"
                  className={cn(
                    'relative justify-start gap-3 px-3 font-normal text-sm',
                    activeView === 'tools' && activeCategory === cat.id
                      ? 'bg-accent text-accent-foreground'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                  onClick={() => onSelectCategory(cat.id)}
                >
                  {activeView === 'tools' && activeCategory === cat.id && (
                    <span className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary" />
                  )}
                  <cat.icon className="h-4 w-4" />
                  {t(cat.label)}
                </Button>
              ))}
            </div>
          )}
        </div>

        <div className="border-t border-border p-3">
          <Button
            variant="ghost"
            className="justify-start gap-3 px-3 w-full font-normal text-muted-foreground hover:text-foreground"
            onClick={onSettings}
          >
            <Settings className="h-4 w-4" />
            {t('sidebar.settings')}
          </Button>
        </div>

        <Dialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('sidebar.conversations.deleteTitle')}</DialogTitle>
              <DialogDescription>
                {t('sidebar.conversations.deleteConfirm', { title: deleteTarget?.title || '' })}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={handleConfirmDelete}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </aside>
    )
  }

  return (
    <aside className="flex w-12 flex-col border-r border-border bg-background overflow-hidden transition-all duration-200">
      <div className="flex flex-col items-center gap-2 py-3 border-b border-border">
        <div className="relative">
          <Button
            variant="ghost"
            size="icon"
            className={cn(
              'h-9 w-9',
              activeView === 'chat'
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={onNewConversation}
            onMouseEnter={() => setTooltip('sidebar.conversations.new')}
            onMouseLeave={() => setTooltip(null)}
          >
            <MessageSquare className="h-4 w-4" />
          </Button>
          {tooltip === 'sidebar.conversations.new' && (
            <div className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1 text-xs text-popover-foreground shadow-sm">
              {t('sidebar.conversations.new')}
            </div>
          )}
        </div>
      </div>

      <div className="flex flex-1 flex-col items-center gap-2 py-3 overflow-y-auto">
        {categories.map((cat) => (
          <div key={cat.id} className="relative">
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                'h-9 w-9',
                activeView === 'tools' && activeCategory === cat.id
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

      <div className="border-t border-border py-3 flex justify-center">
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
