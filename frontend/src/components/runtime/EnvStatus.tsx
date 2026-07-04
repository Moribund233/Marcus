import { XCircle, CheckCircle2, RefreshCw, AlertCircle } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { Button } from '@/components/ui/button'
import type { RuntimeInfo } from '@/components/renderer/types'

interface EnvStatusProps {
  status: Record<string, RuntimeInfo>
  loading: boolean
  onRefresh: () => void
  onClose?: () => void
}

export function EnvStatus({ status, loading, onRefresh, onClose }: EnvStatusProps) {
  const { t } = useI18n()
  const entries = Object.entries(status)
  const allAvailable = entries.every(([, info]) => info.available)

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
          {entries.map(([key, info]) => (
            <div
              key={key}
              className="flex items-center gap-4 rounded-xl border border-border bg-card p-4"
            >
              <div className={info.available ? 'text-emerald-500' : 'text-muted-foreground/50'}>
                {info.available
                  ? <CheckCircle2 className="h-5 w-5" />
                  : <AlertCircle className="h-5 w-5" />
                }
              </div>
              <div className="flex-1">
                <div className="text-sm font-medium">{info.name}</div>
                {info.available ? (
                  <div className="text-xs text-muted-foreground">
                    {info.version && <span>v{info.version} · </span>}
                    {info.path}
                  </div>
                ) : (
                  <div className="text-xs text-muted-foreground/60">{t('env.notInstalled')}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
