import { useState, useEffect, useCallback } from 'react'
import { Package, Code2, ExternalLink, FileText, X, Loader2 } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { Button } from '@/components/ui/button'
import { GetAppLogs } from '../../../wailsjs/go/main/App'

export function AboutTab() {
  const { t } = useI18n()
  const [showLogs, setShowLogs] = useState(false)
  const [logs, setLogs] = useState<string[]>([])
  const [loadingLogs, setLoadingLogs] = useState(false)

  const handleOpenLogs = useCallback(async () => {
    setShowLogs(true)
    setLoadingLogs(true)
    try {
      const result = await GetAppLogs(500)
      setLogs(result ?? [])
    } catch {
      setLogs([])
    } finally {
      setLoadingLogs(false)
    }
  }, [])

  useEffect(() => {
    if (showLogs) {
      const el = document.getElementById('log-viewer-bottom')
      el?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs, showLogs])

  return (
    <div className="mx-auto max-w-lg p-6 pt-8">
      <div className="flex flex-col items-center gap-4 py-8">
        <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl bg-primary/10">
          <img src="/icon.png" alt="Marcus" className="h-12 w-12" />
        </div>
        <div className="text-center">
          <h2 className="text-xl font-semibold tracking-tight">Marcus</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t('about.desc')}</p>
          <p className="mt-0.5 text-xs text-muted-foreground/60">v0.1.0</p>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3">
          <div className="flex items-center gap-3">
            <Package className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('about.techStack')}</div>
              <div className="text-xs text-muted-foreground">{t('about.techStackDesc')}</div>
            </div>
          </div>
        </div>

        <a
          href="https://github.com/Moribund233/Marcus"
          target="_blank"
          className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3 transition-colors hover:bg-accent"
        >
          <div className="flex items-center gap-3">
            <Code2 className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('about.sourceCode')}</div>
              <div className="text-xs text-muted-foreground">{t('about.sourceCodeDesc')}</div>
            </div>
          </div>
          <ExternalLink className="h-4 w-4 text-muted-foreground/60" />
        </a>

        <a
          href="https://github.com/Moribund233/Marcus-plugins"
          target="_blank"
          className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3 transition-colors hover:bg-accent"
        >
          <div className="flex items-center gap-3">
            <Package className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('about.pluginRepo')}</div>
              <div className="text-xs text-muted-foreground">{t('about.pluginRepoDesc')}</div>
            </div>
          </div>
          <ExternalLink className="h-4 w-4 text-muted-foreground/60" />
        </a>

        <button
          onClick={handleOpenLogs}
          className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3 transition-colors hover:bg-accent"
        >
          <div className="flex items-center gap-3">
            <FileText className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('about.viewLogs')}</div>
              <div className="text-xs text-muted-foreground">~/.marcus/app.log</div>
            </div>
          </div>
          <ExternalLink className="h-4 w-4 text-muted-foreground/60" />
        </button>
      </div>

      <p className="mt-8 text-center text-xs text-muted-foreground/40">
        {t('about.footer')}
      </p>

      {/* Log Viewer Dialog */}
      {showLogs && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 flex h-[70vh] w-full max-w-2xl flex-col rounded-xl border border-border bg-card shadow-xl">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h3 className="text-sm font-medium">{t('about.logsTitle')}</h3>
              <Button variant="ghost" size="icon" onClick={() => setShowLogs(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="flex-1 overflow-y-auto p-4 font-mono text-xs leading-relaxed">
              {loadingLogs ? (
                <div className="flex items-center justify-center gap-2 py-12 text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span>Loading...</span>
                </div>
              ) : logs.length === 0 ? (
                <div className="py-12 text-center text-muted-foreground/60">
                  {t('about.logsEmpty')}
                </div>
              ) : (
                logs.map((line, i) => (
                  <div key={i} className="whitespace-pre-wrap break-all text-muted-foreground hover:text-foreground">
                    {line}
                  </div>
                ))
              )}
              <div id="log-viewer-bottom" />
            </div>
            <div className="flex justify-end border-t border-border px-5 py-3">
              <Button variant="outline" size="sm" onClick={() => setShowLogs(false)}>
                {t('about.close')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
