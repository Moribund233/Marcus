import { useState, useCallback, useEffect, useRef } from 'react'
import { FileUp, Package, Loader2, CheckCircle2, AlertCircle } from 'lucide-react'
import { OpenFileDialog, InstallToolPackageAsync } from '../../../../wailsjs/go/main/App'
import { EventsOn } from '../../../../wailsjs/runtime'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'

interface InstallProgress {
  status: 'idle' | 'starting' | 'running' | 'warning' | 'error'
  message: string
  progress: number
}

interface InstallTabProps {
  onInstalled: () => void
}

export function InstallTab({ onInstalled }: InstallTabProps) {
  const { t } = useI18n()
  const [filePath, setFilePath] = useState('')
  const [dragging, setDragging] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [result, setResult] = useState<'success' | 'error' | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const [progress, setProgress] = useState<InstallProgress>({ status: 'idle', message: '', progress: 0 })
  const currentPathRef = useRef('')

  useEffect(() => {
    const unsubProgress = EventsOn('install:progress', (data: { path: string; status: string; message: string; progress: number }) => {
      if (data.path !== currentPathRef.current) return
      setProgress({
        status: data.status as InstallProgress['status'],
        message: data.message,
        progress: data.progress,
      })
    })
    const unsubComplete = EventsOn('install:complete', (data: { path: string; success: boolean; error: string }) => {
      if (data.path !== currentPathRef.current) return
      setInstalling(false)
      if (data.success) {
        setResult('success')
        setFilePath('')
        setProgress({ status: 'idle', message: '', progress: 0 })
        onInstalled()
      } else {
        setResult('error')
        setErrorMsg(data.error)
        setProgress({ status: 'error', message: data.error, progress: 0 })
      }
    })
    return () => {
      unsubProgress()
      unsubComplete()
    }
  }, [onInstalled])

  const handleSelect = useCallback(async () => {
    const path = await OpenFileDialog('*.whl;*.tgz;*.tar.gz')
    if (path) {
      setFilePath(path)
      setResult(null)
      setErrorMsg('')
      setProgress({ status: 'idle', message: '', progress: 0 })
    }
  }, [])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    const file = e.dataTransfer.files[0]
    if (file && 'path' in file && (file as any).path) {
      setFilePath((file as any).path)
      setResult(null)
      setErrorMsg('')
      setProgress({ status: 'idle', message: '', progress: 0 })
    }
  }, [])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragging(true)
  }, [])

  const handleDragLeave = useCallback(() => {
    setDragging(false)
  }, [])

  const handleInstall = useCallback(async () => {
    if (!filePath) return
    currentPathRef.current = filePath
    setInstalling(true)
    setResult(null)
    setErrorMsg('')
    setProgress({ status: 'starting', message: t('toolAdd.install.preparing'), progress: 0 })
    try {
      await InstallToolPackageAsync(filePath)
    } catch (e) {
      setInstalling(false)
      setResult('error')
      setErrorMsg(String(e))
      setProgress({ status: 'error', message: String(e), progress: 0 })
    }
  }, [filePath, t])

  const fileName = filePath ? filePath.split(/[\\/]/).pop() : ''

  return (
    <div className="mx-auto max-w-lg pt-8">
      <div
        className={`relative flex cursor-pointer flex-col items-center rounded-2xl border-2 border-dashed p-12 text-center transition-colors ${
          dragging
            ? 'border-primary bg-primary/5'
            : 'border-border hover:border-primary/30'
        }`}
        onClick={!filePath ? handleSelect : undefined}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
      >
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          {filePath ? <Package className="h-8 w-8" /> : <FileUp className="h-8 w-8" />}
        </div>

        {!filePath ? (
          <>
            <p className="text-base font-medium">{t('toolAdd.install.dropTitle')}</p>
            <p className="mt-1.5 text-sm text-muted-foreground">
              {t('toolAdd.install.dropHint')}
            </p>
            <Button variant="outline" size="sm" className="mt-5" onClick={(e) => { e.stopPropagation(); handleSelect() }}>
              {t('toolAdd.install.browse')}
            </Button>
          </>
        ) : (
          <>
            <p className="text-base font-medium">{fileName}</p>
            <p className="mt-1 text-xs text-muted-foreground">{filePath}</p>

            <div className="mt-6 flex gap-3">
              <Button onClick={(e) => { e.stopPropagation(); handleInstall() }} disabled={installing}>
                {installing ? (
                  <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                ) : (
                  <Package className="mr-1.5 h-4 w-4" />
                )}
                {installing ? t('toolAdd.install.installing') : t('toolAdd.install.install')}
              </Button>
              <Button variant="ghost" onClick={(e) => { e.stopPropagation(); setFilePath('') }}>
                {t('toolAdd.install.clear')}
              </Button>
            </div>
          </>
        )}

        {installing && progress.status !== 'idle' && (
          <div className="mt-6 w-full max-w-xs">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{progress.message}</span>
              <span>{Math.round(progress.progress)}%</span>
            </div>
            <div className="mt-2 h-2 overflow-hidden rounded-full bg-secondary">
              <div
                className="h-full rounded-full bg-primary transition-all duration-300 ease-out"
                style={{ width: `${Math.min(progress.progress, 100)}%` }}
              />
            </div>
          </div>
        )}

        {result === 'success' && (
          <div className="mt-4 flex items-center gap-2 text-sm text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-4 w-4" />
            {t('toolAdd.install.success')}
          </div>
        )}
        {result === 'error' && (
          <div className="mt-4 flex items-center gap-2 text-sm text-red-600 dark:text-red-400">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span className="break-all">{errorMsg}</span>
          </div>
        )}
      </div>
    </div>
  )
}
