import { Minus, Square, X, PanelLeft, PanelRight, PanelBottom } from 'lucide-react'
import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'

interface TitleBarProps {
  sidebarExpanded: boolean
  rightSidebarVisible: boolean
  statusBarVisible: boolean
  onToggleSidebar: () => void
  onToggleRightSidebar: () => void
  onToggleStatusBar: () => void
}

export function TitleBar({
  sidebarExpanded,
  rightSidebarVisible,
  statusBarVisible,
  onToggleSidebar,
  onToggleRightSidebar,
  onToggleStatusBar,
}: TitleBarProps) {
  const { t } = useI18n()
  return (
    <div
      className="flex h-11 items-center justify-between border-b border-border select-none bg-background/80 backdrop-blur-sm"
      style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
    >
      <span className="ml-4 text-sm font-mono font-semibold tracking-wide text-foreground/80 select-none">
        Marcus
      </span>

      <div className="flex items-center" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
        <Button
          variant="ghost"
          size="icon"
          className={cn('h-11 w-10 rounded-none', sidebarExpanded ? 'text-foreground/80' : 'text-muted-foreground/50')}
          onClick={onToggleSidebar}
          title={sidebarExpanded ? t('titleBar.collapseSidebar') : t('titleBar.expandSidebar')}
        >
          <PanelLeft className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className={cn('h-11 w-10 rounded-none', rightSidebarVisible ? 'text-foreground/80' : 'text-muted-foreground/50')}
          onClick={onToggleRightSidebar}
          title={rightSidebarVisible ? t('titleBar.hideRightSidebar') : t('titleBar.showRightSidebar')}
        >
          <PanelRight className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className={cn('h-11 w-10 rounded-none', statusBarVisible ? 'text-foreground/80' : 'text-muted-foreground/50')}
          onClick={onToggleStatusBar}
          title={statusBarVisible ? t('titleBar.hideStatusBar') : t('titleBar.showStatusBar')}
        >
          <PanelBottom className="h-4 w-4" />
        </Button>
        <div className="mx-1 h-5 w-px bg-border" />
        <Button
          variant="ghost"
          size="icon"
          className="h-11 w-11 rounded-none hover:bg-accent"
          onClick={WindowMinimise}
        >
          <Minus className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-11 w-11 rounded-none hover:bg-accent"
          onClick={WindowToggleMaximise}
        >
          <Square className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-11 w-11 rounded-none hover:bg-destructive hover:text-destructive-foreground"
          onClick={Quit}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}
