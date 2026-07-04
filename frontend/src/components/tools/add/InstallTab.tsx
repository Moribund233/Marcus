import { useState, useCallback } from 'react'
import { FileUp, Package, Loader2, CheckCircle2, AlertCircle } from 'lucide-react'
import { OpenFileDialog, InstallToolPackage } from '../../../../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'

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

  const handleSelect = useCallback(async () => {
    const path = await OpenFileDialog('*.whl;*.tgz;*.tar.gz')
    if (path) {
      setFilePath(path)
      setResult(null)
      setErrorMsg('')
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
    setInstalling(true)
    setResult(null)
    setErrorMsg('')
    try {
      await InstallToolPackage(filePath)
      setResult('success')
      setFilePath('')
      onInstalled()
    } catch (e) {
      setResult('error')
      setErrorMsg(String(e))
    } finally {
      setInstalling(false)
    }
  }, [filePath, onInstalled])

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
