import { useState } from 'react'
import { ArrowLeft, Settings2, Monitor, Sliders, Keyboard, Info } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { EnvStatus } from '@/components/runtime/EnvStatus'
import { GeneralTab } from '@/components/settings/GeneralTab'
import { SandboxTab } from '@/components/settings/SandboxTab'
import { ShortcutsTab } from '@/components/settings/ShortcutsTab'
import { AboutTab } from '@/components/settings/AboutTab'
import type { RuntimeInfo } from '@/components/renderer/types'

type SettingsTab = 'general' | 'environment' | 'sandbox' | 'shortcuts' | 'about'

interface SettingsProps {
  runtimeStatus: Record<string, RuntimeInfo>
  runtimeLoading: boolean
  onRefreshRuntime: () => void
  onClose: () => void
}

export function Settings({ runtimeStatus, runtimeLoading, onRefreshRuntime, onClose }: SettingsProps) {
  const { t } = useI18n()
  const [tab, setTab] = useState<SettingsTab>('general')

  const TABS: { id: SettingsTab; label: string; icon: React.ReactNode }[] = [
    { id: 'general', label: t('settings.general'), icon: <Settings2 className="h-4 w-4" /> },
    { id: 'environment', label: t('settings.environment'), icon: <Monitor className="h-4 w-4" /> },
    { id: 'sandbox', label: t('settings.sandbox'), icon: <Sliders className="h-4 w-4" /> },
    { id: 'shortcuts', label: t('settings.shortcuts'), icon: <Keyboard className="h-4 w-4" /> },
    { id: 'about', label: t('settings.about'), icon: <Info className="h-4 w-4" /> },
  ]

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b border-border px-6 py-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={onClose}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-base font-medium">{t('settings.title')}</h1>
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
        {tab === 'general' && <GeneralTab />}
        {tab === 'environment' && (
          <EnvStatus
            status={runtimeStatus}
            loading={runtimeLoading}
            onRefresh={onRefreshRuntime}
          />
        )}
        {tab === 'sandbox' && <SandboxTab />}
        {tab === 'shortcuts' && <ShortcutsTab />}
        {tab === 'about' && <AboutTab />}
      </div>
    </div>
  )
}
