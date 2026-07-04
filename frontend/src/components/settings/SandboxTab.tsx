import { Timer, Cpu, MemoryStick, AlertTriangle } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { useConfig } from '@/hooks/useConfig'

const noop = () => {}

export function SandboxTab() {
  const { t } = useI18n()
  const { config, save } = useConfig()
  const timeout = config?.default_timeout_seconds ?? 0
  const cpu = config?.default_cpu_percent ?? 0
  const memory = config?.default_memory_mb ?? 0

  return (
    <div className="mx-auto max-w-lg p-6 pt-8">
      <h2 className="text-base font-medium">{t('sandbox.title')}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{t('sandbox.desc')}</p>

      <div className="mt-8 flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Timer className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('sandbox.timeout')}</div>
              <div className="text-xs text-muted-foreground">{t('sandbox.timeoutDesc')}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <input
              className="w-20 rounded-lg border border-border bg-card px-3 py-1.5 text-sm text-right outline-none transition-colors focus:border-primary/50 font-mono"
              type="number"
              value={timeout}
              min={0}
              onChange={(e) => save({ default_timeout_seconds: Math.max(0, parseInt(e.target.value) || 0) })}
            />
            <span className="text-xs text-muted-foreground">{t('sandbox.timeoutUnit')}</span>
          </div>
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Cpu className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('sandbox.cpu')}</div>
              <div className="text-xs text-muted-foreground">{t('sandbox.cpuDesc')}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <input
              className="w-20 rounded-lg border border-border bg-card px-3 py-1.5 text-sm text-right outline-none transition-colors focus:border-primary/50 font-mono"
              type="number"
              value={cpu}
              min={0}
              max={100}
              onChange={(e) => save({ default_cpu_percent: Math.min(100, Math.max(0, parseInt(e.target.value) || 0)) })}
            />
            <span className="text-xs text-muted-foreground">{t('sandbox.cpuUnit')}</span>
          </div>
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <MemoryStick className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('sandbox.memory')}</div>
              <div className="text-xs text-muted-foreground">{t('sandbox.memoryDesc')}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <input
              className="w-20 rounded-lg border border-border bg-card px-3 py-1.5 text-sm text-right outline-none transition-colors focus:border-primary/50 font-mono"
              type="number"
              value={memory}
              min={0}
              onChange={(e) => save({ default_memory_mb: Math.max(0, parseInt(e.target.value) || 0) })}
            />
            <span className="text-xs text-muted-foreground">{t('sandbox.memoryUnit')}</span>
          </div>
        </div>

        <div className="mt-2 flex items-start gap-3 rounded-lg border border-amber-500/20 bg-amber-500/5 p-3">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
          <p className="text-xs text-muted-foreground">
            {t('sandbox.warning')}
          </p>
        </div>
      </div>
    </div>
  )
}
