import { useState, useEffect, useCallback } from 'react'
import { ArrowLeft, AlertTriangle, Play, Square, Terminal, Globe, File, FolderOpen, FileUp, Trash2 } from 'lucide-react'
import { model } from '../../../wailsjs/go/models'
import { EventsOn } from '../../../wailsjs/runtime'
import { OpenFileDialog, OpenDirectoryDialog } from '../../../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { TerminalView } from '@/components/terminal/TerminalView'
import { FormRenderer } from '@/components/renderer/FormRenderer'
import { OutputRenderer } from '@/components/renderer/OutputRenderer'
import { FileSelector } from '@/components/file/FileSelector'
import { ProgressView } from '@/components/file/ProgressView'
import type { ToolManifest, TerminalManifest, FileManifest } from '@/components/renderer/types'
import { useI18n } from '@/hooks/useI18n'

interface ToolDetailProps {
  tool: model.ToolInfo
  onBack: () => void
  onLaunch: (id: string, args: Record<string, string>) => Promise<model.ProcessState>
  onStop: (id: string) => Promise<void>
  onUninstall: (id: string) => Promise<model.UninstallResult>
  onToolLaunch: (tool: model.ToolInfo) => void
  onToolStop: (toolId: string) => void
}

interface ArgField {
  name: string
  label: string
  type: string
  default?: unknown
}

export function ToolDetail({ tool, onBack, onLaunch, onStop, onUninstall, onToolLaunch, onToolStop }: ToolDetailProps) {
  const { t } = useI18n()
  const [state, setState] = useState<model.ProcessState | null>(null)
  const [output, setOutput] = useState<string[]>([])
  const MAX_OUTPUT_LINES = 1000
  const [args, setArgs] = useState<Record<string, string>>({})
  const [progress, setProgress] = useState(0)
  const [polling, setPolling] = useState(false)

  let manifest: ToolManifest | null = null
  try {
    manifest = JSON.parse(tool.manifest)
  } catch {}

  const argFields: ArgField[] = manifest
    ? manifest.terminal?.args ?? manifest.file?.args ?? []
    : []

  useEffect(() => {
    const unsub = EventsOn('tool:output', (data: { tool_id: string; line: string }) => {
      if (data.tool_id === tool.id) {
        setOutput((prev) => {
          if (prev.length >= MAX_OUTPUT_LINES) {
            return [...prev.slice(-MAX_OUTPUT_LINES / 2), data.line]
          }
          return [...prev, data.line]
        })
      }
    })
    return () => unsub()
  }, [tool.id])

  // listen to backend-pushed state changes (replaces polling).
  useEffect(() => {
    const unsub = EventsOn('tool:state-changed', (data: { tool_id: string; status: string; exit_code?: number; error?: string }) => {
      if (data.tool_id !== tool.id) return
      setState((prev) => {
        if (!prev) return null
        return model.ProcessState.createFrom({
          ...prev,
          status: data.status,
          exit_code: data.exit_code ?? 0,
          error_log: data.error ?? '',
        })
      })
      if (data.status === 'exited' || data.status === 'crashed') {
        setPolling(false)
        onToolStop(tool.id)
      }
    })
    return () => unsub()
  }, [tool.id, onToolStop])

  const setArg = (name: string, value: string) => {
    setArgs((prev) => ({ ...prev, [name]: value }))
  }

  const handleFileSelect = async (field: ArgField) => {
    const path = field.type === 'directory'
      ? await OpenDirectoryDialog()
      : await OpenFileDialog('*.*')
    if (path) {
      setArg(field.name, path)
    }
  }

  const handleLaunch = async () => {
    const s = await onLaunch(tool.id, args)
    setState(s)
    setOutput([])
    setProgress(0)
    setPolling(true)
    onToolLaunch(tool)
  }

  const handleStop = async () => {
    setPolling(false)
    await onStop(tool.id)
    onToolStop(tool.id)
    setState((prev) => prev ? model.ProcessState.createFrom({ ...prev, status: 'stopping' }) : null)
  }

  const [uninstalling, setUninstalling] = useState(false)

  const handleUninstallConfirm = async () => {
    setUninstalling(true)
    try {
      const result = await onUninstall(tool.id)
      if (result.success) {
        onBack()
      }
    } finally {
      setUninstalling(false)
    }
  }

  const contributionIcon = () => {
    switch (tool.contribution) {
      case 'web': return <Globe className="h-4 w-4" />
      case 'terminal': return <Terminal className="h-4 w-4" />
      case 'file': return <File className="h-4 w-4" />
      default: return null
    }
  }

  const isRunning = polling || state?.status === 'running' || state?.status === 'launching'

  return (
    <div className="flex flex-1 flex-col overflow-y-auto">
      <div className="flex items-center gap-3 border-b border-border px-6 py-4">
        <Button variant="ghost" size="icon" onClick={onBack} className="shrink-0">
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h2 className="text-lg font-medium">{tool.display_name}</h2>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              {contributionIcon()}
              {tool.contribution}
            </span>
            <span>·</span>
            <span>{tool.source}</span>
            {tool.version && <><span>·</span><span>v{tool.version}</span></>}
          </div>
        </div>
        <div className="ml-auto flex gap-2">
          {!isRunning ? (
            <>
              <Button onClick={handleLaunch} disabled={state?.status === 'launching'}>
                <Play className="mr-1.5 h-4 w-4" />
                {t('toolDetail.launch')}
              </Button>
              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Trash2 className="mr-1.5 h-4 w-4" />
                    {t('toolDetail.uninstall')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-md">
                  <DialogHeader>
                    <DialogTitle>{t('toolDetail.uninstall')}</DialogTitle>
                    <DialogDescription>
                      {t('toolDetail.uninstallConfirm', { name: tool.display_name })}
                    </DialogDescription>
                  </DialogHeader>
                  <DialogFooter>
                    <Button variant="outline">{t('toolAddManual.cancel')}</Button>
                    <Button variant="destructive" onClick={handleUninstallConfirm} disabled={uninstalling}>
                      {uninstalling ? `${t('toolDetail.uninstall')}...` : t('toolDetail.uninstall')}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </>
          ) : (
            <>
              <Button variant="destructive" onClick={handleStop}>
                <Square className="mr-1.5 h-4 w-4" />
                {t('toolDetail.stop')}
              </Button>
              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Trash2 className="mr-1.5 h-4 w-4" />
                    {t('toolDetail.uninstall')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="sm:max-w-md">
                  <DialogHeader>
                    <DialogTitle>{t('toolDetail.uninstall')}</DialogTitle>
                    <DialogDescription>
                      {t('toolDetail.uninstallConfirm', { name: tool.display_name })}
                    </DialogDescription>
                  </DialogHeader>
                  <DialogFooter>
                    <Button variant="outline">{t('toolAddManual.cancel')}</Button>
                    <Button variant="destructive" onClick={handleUninstallConfirm} disabled={uninstalling}>
                      {uninstalling ? `${t('toolDetail.uninstall')}...` : t('toolDetail.uninstall')}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </>
          )}
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-6 p-6">
        {tool.description && (
          <p className="text-sm text-muted-foreground">{tool.description}</p>
        )}

        {'healthy' in tool && tool.healthy === false && tool.health_error && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
              <div className="text-sm">
                <p className="font-medium text-amber-600">{tool.health_error}</p>
                {tool.health_hint && (
                  <pre className="mt-2 whitespace-pre-wrap text-xs text-muted-foreground">{tool.health_hint}</pre>
                )}
              </div>
            </div>
          </div>
        )}

        {manifest?.ui?.inputs && manifest.ui.inputs.length > 0 && (
          <FormRenderer inputs={manifest.ui.inputs} values={args} onChange={setArg} />
        )}

        {argFields.length > 0 && (
          <div className="flex flex-col gap-4">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t('toolDetail.arguments')}
            </h3>
            {argFields.map((field) => (
              <div key={field.name} className="flex flex-col gap-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  {field.label}
                </label>
                {field.type === 'file' || field.type === 'directory' ? (
                  <div className="flex items-center gap-2">
                    <input
                      className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
                      type="text"
                      value={args[field.name] ?? ''}
                      readOnly
                      placeholder={field.type === 'directory' ? t('file.selectDirHint') : t('file.selectFileHint')}
                    />
                    <Button variant="outline" size="sm" onClick={() => handleFileSelect(field)}>
                      {field.type === 'directory' ? <FolderOpen className="h-4 w-4" /> : <FileUp className="h-4 w-4" />}
                      {t('file.browse')}
                    </Button>
                  </div>
                ) : (
                  <input
                    className="rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
                    type={field.type === 'number' ? 'number' : 'text'}
                    defaultValue={String(field.default ?? '')}
                    onChange={(e) => setArg(field.name, e.target.value)}
                  />
                )}
              </div>
            ))}
          </div>
        )}

        {tool.contribution === 'file' && manifest?.file && (
          <FileSelector
            inputType={(manifest.file as FileManifest).input_type}
            extensions={(manifest.file as FileManifest).input_extensions}
            onSelect={(paths) => setArg('input', paths[0])}
          />
        )}

        {progress > 0 && progress < 100 && <ProgressView progress={progress} />}

        {state && state.status === 'running' && tool.contribution === 'web' && state.port && state.port > 0 && (
          <div className="rounded-lg border border-primary/30 bg-primary/5 p-4 text-sm text-primary">
            {t('toolDetail.serviceStarted', { port: state.port })}
          </div>
        )}

        {(tool.contribution === 'terminal' || tool.contribution === 'file') && (
          <TerminalView lines={output} />
        )}

        {manifest?.ui?.output && state?.status === 'exited' && (
          <OutputRenderer fields={manifest.ui.output} />
        )}
      </div>
    </div>
  )
}
