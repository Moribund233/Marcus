import { useState } from 'react'
import { Package, Store, Terminal, ArrowLeft } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { InstallTab } from '@/components/tools/add/InstallTab'
import { MarketTab } from '@/components/tools/add/MarketTab'
import { CliTab } from '@/components/tools/add/CliTab'

type AddTab = 'market' | 'install' | 'cli'

interface ToolAddProps {
  onAdd: (name: string, command: string, argType: string) => void
  onCancel: () => void
  onRefresh: () => void
}

export function ToolAdd({ onAdd, onCancel, onRefresh }: ToolAddProps) {
  const { t } = useI18n()
  const [tab, setTab] = useState<AddTab>('market')

  const TABS: { id: AddTab; label: string; icon: React.ReactNode }[] = [
    { id: 'market', label: t('toolAdd.market.title'), icon: <Store className="h-4 w-4" /> },
    { id: 'install', label: t('toolAdd.install.title'), icon: <Package className="h-4 w-4" /> },
    { id: 'cli', label: t('toolAdd.cli.title'), icon: <Terminal className="h-4 w-4" /> },
  ]

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b border-border px-6 py-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={onCancel}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-base font-medium">{t('toolAdd.title')}</h1>
        </div>

        <div className="flex items-center gap-1">
          {TABS.map((item) => (
            <button
              key={item.id}
              className={cn(
                'flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm transition-colors',
                tab === item.id
                  ? 'bg-accent text-accent-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground',
              )}
              onClick={() => setTab(item.id)}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {tab === 'market' && <MarketTab onInstallSuccess={onRefresh} />}
        {tab === 'install' && <InstallTab onInstalled={onRefresh} />}
        {tab === 'cli' && <CliTab onAdd={onAdd} onCancel={onCancel} />}
      </div>
    </div>
  )
}
