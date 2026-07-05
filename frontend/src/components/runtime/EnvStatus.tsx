import { useState, useEffect, useCallback } from 'react'
import { XCircle, CheckCircle2, RefreshCw, AlertCircle, Loader2, Download } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { Button } from '@/components/ui/button'
import { InstallRuntimeAsync } from '../../../wailsjs/go/main/App'
import { EventsOn } from '../../../wailsjs/runtime'
import type { RuntimeInfo } from '@/components/renderer/types'

interface EnvStatusProps {
  status: Record<string, RuntimeInfo>
  loading: boolean
  onRefresh: () => void
  onClose?: () => void
}

interface InstallProgress {
  runtime: string
  status: string
  message: string
  progress: number
}

const INSTALLABLE = ['uv', 'bun']

export function EnvStatus({ status, loading, onRefresh, onClose }: EnvStatusProps) {
  const { t } = useI18n()
  const [installProgress, setInstallProgress] = useState<Record<string, InstallProgress>>({})
  const entries = Object.entries(status)
  const allAvailable = entries.every(([, info]) => info.available)

  useEffect(() => {
    const unsubProgress = EventsOn('runtime:install-progress', (data: InstallProgress) => {
      setInstallProgress((prev) => ({ ...prev, [data.runtime]: data }))
    })
    const unsubComplete = EventsOn('runtime:install-complete', (data: { runtime: string; success: boolean }) => {
      setInstallProgress((prev) => {
        const next = { ...prev }
        delete next[data.runtime]
        return next
      })
      if (data.success) {
        onRefresh()
      }
    })
    return () => {
      unsubProgress()
      unsubComplete()
    }
  }, [onRefresh])

  const handleInstall = useCallback((runtime: string) => {
    InstallRuntimeAsync(runtime)
  }, [])

  return (
    <div className="flex flex-1 items-start justify-center overflow-y-auto p-6 pt-12">
      <div className="w-full max-w-lg">
        <div className="mb-8 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-medium">{t('env.title')}</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {allAvailable ? t('env.allReady') : t('env.partialMissing')}
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="ghost" size="icon" onClick={onRefresh} disabled={loading}>
              <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            {onClose && (
              <Button variant="ghost" size="icon" onClick={onClose}>
                <XCircle className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-3">
          {entries.map(([key, info]) => {
            const prog = installProgress[key]
            return (
              <div
                key={key}
                className="flex items-center gap-4 rounded-xl border border-border bg-card p-4"
              >
                <div className={info.available ? 'text-emerald-500' : 'text-muted-foreground/50'}>
                  {prog ? (
                    <Loader2 className="h-5 w-5 animate-spin" />
                  ) : info.available ? (
                    <CheckCircle2 className="h-5 w-5" />
                  ) : (
                    <AlertCircle className="h-5 w-5" />
                  )}
                </div>
                <div className="flex-1">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-medium">{info.name}</div>
                    {!info.available && !prog && INSTALLABLE.includes(key) && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 gap-1 text-xs"
                        onClick={() => handleInstall(key)}
                      >
                        <Download className="h-3 w-3" />
                        安装
                      </Button>
                    )}
                  </div>
                  {prog ? (
                    <div className="mt-1 space-y-1">
                      <div className="text-xs text-muted-foreground">
                        {prog.message}
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-secondary">
                        <div
                          className="h-full rounded-full bg-primary transition-all duration-300 ease-out"
                          style={{ width: `${Math.min(prog.progress, 100)}%` }}
                        />
                      </div>
                    </div>
                  ) : info.available ? (
                    <div className="text-xs text-muted-foreground">
                      {info.version && <span>v{info.version} · </span>}
                      {info.path}
                    </div>
                  ) : (
                    <div className="text-xs text-muted-foreground/60">{t('env.notInstalled')}</div>
                  )}
                  {!prog && info.hint && (
                    <div className="mt-1.5 whitespace-pre-wrap rounded-md bg-amber-500/10 px-2.5 py-1.5 text-xs text-amber-600 dark:text-amber-400">
                      {info.hint}
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
