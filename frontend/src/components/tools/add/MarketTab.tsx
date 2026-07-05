import { useState, useEffect, useCallback } from 'react'
import {
  Store,
  RefreshCw,
  Download,
  CheckCircle2,
  Loader2,
  AlertCircle,
  Search,
  AlertTriangle,
  Archive,
} from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { Button } from '@/components/ui/button'
import {
  StoreSync,
  StoreListPlugins,
  StoreSearchPlugins,
  StoreInstall,
} from '../../../../wailsjs/go/main/App'
import { model } from '../../../../wailsjs/go/models'
import { ErrorDialog } from '@/components/common/ErrorDialog'

type TFunction = (key: string, params?: Record<string, string | number>) => string

function parseError(error: string, t: TFunction): string {
  if (error.includes('TLS handshake timeout') || error.includes('timeout')) {
    return t('toolAdd.market.error.networkTimeout')
  }
  if (error.includes('connection refused') || error.includes('net/http')) {
    return t('toolAdd.market.error.connectionFailed')
  }
  if (error.includes('404') || error.includes('not found')) {
    return t('toolAdd.market.error.notFound')
  }
  if (error.includes('403') || error.includes('forbidden')) {
    return t('toolAdd.market.error.forbidden')
  }
  if (error.includes('500') || error.includes('server error')) {
    return t('toolAdd.market.error.serverError')
  }
  if (error.includes('download')) {
    return t('toolAdd.market.error.downloadFailed')
  }
  if (error.includes('bun not found')) {
    return t('toolAdd.market.error.bunNotFound')
  }
  if (error.includes('uv not found')) {
    return t('toolAdd.market.error.uvNotFound')
  }
  return t('toolAdd.market.error.unknown')
}

interface MarketTabProps {
  onInstallSuccess?: () => void
}

export function MarketTab({ onInstallSuccess }: MarketTabProps) {
  const { t } = useI18n()
  const [plugins, setPlugins] = useState<model.StorePlugin[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [installing, setInstalling] = useState<Record<string, boolean>>({})
  const [statusMsg, setStatusMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [errorDialog, setErrorDialog] = useState<{ open: boolean; title: string; message: string; details?: string }>({
    open: false,
    title: '',
    message: '',
  })

  const loadPlugins = useCallback(async (query?: string) => {
    try {
      if (query) {
        const results = await StoreSearchPlugins(query)
        setPlugins(results ?? [])
      } else {
        const results = await StoreListPlugins()
        setPlugins(results ?? [])
      }
    } catch (e) {
      console.error('load plugins failed', e)
    }
  }, [])

  const handleSync = useCallback(async () => {
    setSyncing(true)
    setStatusMsg(null)
    try {
      await StoreSync()
      await loadPlugins()
      setStatusMsg({ type: 'success', text: t('toolAdd.market.syncSuccess') })
    } catch (e) {
      const errMsg = String(e)
      setErrorDialog({
        open: true,
        title: t('toolAdd.market.syncFailed'),
        message: parseError(errMsg, t),
        details: errMsg,
      })
    } finally {
      setSyncing(false)
    }
  }, [loadPlugins, t])

  const handleSearch = useCallback(async (query: string) => {
    setSearchQuery(query)
    await loadPlugins(query || undefined)
  }, [loadPlugins])

  const handleInstall = useCallback(async (pluginId: string, version: string) => {
    setInstalling((prev) => ({ ...prev, [pluginId]: true }))
    setStatusMsg(null)
    try {
      const result = await StoreInstall(pluginId, version)
      if (result.success) {
        setStatusMsg({ type: 'success', text: t('toolAdd.market.installSuccess', { name: pluginId }) })
        await loadPlugins(searchQuery || undefined)
        onInstallSuccess?.()
      } else {
        const errMsg = String(result.error || 'unknown')
        setErrorDialog({
          open: true,
          title: t('toolAdd.market.installFailed'),
          message: parseError(errMsg, t),
          details: errMsg,
        })
      }
    } catch (e) {
      const errMsg = String(e)
      setErrorDialog({
        open: true,
        title: t('toolAdd.market.installFailed'),
        message: parseError(errMsg, t),
        details: errMsg,
      })
    } finally {
      setInstalling((prev) => ({ ...prev, [pluginId]: false }))
    }
  }, [loadPlugins, searchQuery, t, onInstallSuccess])

  useEffect(() => {
    loadPlugins()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const latestVersion = (p: model.StorePlugin) =>
    p.versions?.[p.latest_version] ?? { display_name: p.id, description: '', categories: [] }

  return (
    <div className="flex h-full flex-col">
      {/* Toolbar */}
      <div className="flex items-center gap-3 border-b border-border px-6 py-3">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            className="h-9 w-full rounded-lg border border-input bg-background pl-9 pr-3 text-sm outline-none placeholder:text-muted-foreground focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
            placeholder={t('toolAdd.market.searchPlaceholder')}
            value={searchQuery}
            onChange={(e) => handleSearch(e.target.value)}
          />
        </div>
        <Button variant="outline" size="sm" onClick={handleSync} disabled={syncing}>
          <RefreshCw className={`mr-1.5 h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
          {syncing ? t('toolAdd.market.syncing') : t('toolAdd.market.sync')}
        </Button>
      </div>

      {/* Status message */}
      {statusMsg && (
        <div className={`mx-6 mt-3 flex items-center gap-2 rounded-lg border px-4 py-2 text-sm ${
          statusMsg.type === 'success'
            ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
            : 'border-red-500/20 bg-red-500/10 text-red-700 dark:text-red-400'
        }`}>
          {statusMsg.type === 'success'
            ? <CheckCircle2 className="h-4 w-4 shrink-0" />
            : <AlertCircle className="h-4 w-4 shrink-0" />
          }
          <span>{statusMsg.text}</span>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading && plugins.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 py-20">
            <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted">
              <Store className="h-8 w-8 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground/60">{t('toolAdd.market.syncing')}</p>
          </div>
        ) : plugins.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 py-20">
            <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted">
              <Archive className="h-8 w-8 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground/60">{t('toolAdd.market.empty')}</p>
            <p className="text-xs text-muted-foreground/40">{t('toolAdd.market.emptyHint')}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {plugins.map((plugin) => {
              const latest = latestVersion(plugin)
              const isInstalled = !!plugin.installed_version
              const hasUpdate = plugin.update_available

              return (
                <div
                  key={plugin.id}
                  className="group relative flex flex-col rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/30 hover:shadow-sm"
                >
                  {/* Header */}
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <h3 className="truncate text-sm font-medium text-card-foreground">
                        {latest.display_name || plugin.id}
                      </h3>
                      <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                        {latest.description || plugin.id}
                      </p>
                    </div>
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                      <Store className="h-5 w-5" />
                    </div>
                  </div>

                  {/* Meta */}
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <span className="inline-flex items-center rounded-md bg-secondary px-2 py-0.5 text-[11px] font-medium text-secondary-foreground">
                      {t('toolAdd.market.version')} {plugin.latest_version}
                    </span>
                    {latest.categories?.slice(0, 2).map((cat) => (
                      <span
                        key={cat}
                        className="inline-flex items-center rounded-md bg-secondary/50 px-2 py-0.5 text-[11px] text-muted-foreground"
                      >
                        {cat}
                      </span>
                    ))}
                  </div>

                  {/* Deprecation warning */}
                  {plugin.deprecated && (
                    <div className="mt-2 flex items-center gap-1.5 text-[11px] text-amber-600 dark:text-amber-400">
                      <AlertTriangle className="h-3.5 w-3.5" />
                      <span>{t('toolAdd.market.deprecated')}</span>
                    </div>
                  )}

                  {/* Footer */}
                  <div className="mt-auto pt-4">
                    {isInstalled ? (
                      <div className="flex items-center justify-between">
                        <span className="flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400">
                          <CheckCircle2 className="h-3.5 w-3.5" />
                          {t('toolAdd.market.installed')} {plugin.installed_version}
                        </span>
                        {hasUpdate && !plugin.deprecated && (
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 text-xs"
                            onClick={() => handleInstall(plugin.id, plugin.latest_version)}
                            disabled={installing[plugin.id]}
                          >
                            {installing[plugin.id] ? (
                              <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                            ) : null}
                            {t('toolAdd.market.update')}
                          </Button>
                        )}
                      </div>
                    ) : (
                      <Button
                        size="sm"
                        className="h-8 w-full text-xs"
                        onClick={() => handleInstall(plugin.id, plugin.latest_version)}
                        disabled={installing[plugin.id] || plugin.deprecated}
                      >
                        {installing[plugin.id] ? (
                          <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Download className="mr-1.5 h-3.5 w-3.5" />
                        )}
                        {installing[plugin.id]
                          ? t('toolAdd.market.installing')
                          : t('toolAdd.market.install')}
                      </Button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <ErrorDialog
        open={errorDialog.open}
        onOpenChange={(open) => setErrorDialog((prev) => ({ ...prev, open }))}
        title={errorDialog.title}
        message={errorDialog.message}
        details={errorDialog.details}
      />
    </div>
  )
}
