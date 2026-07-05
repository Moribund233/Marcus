import { useState, useEffect, useRef } from 'react'
import { CheckCircle2, AlertTriangle, Minus, Loader2 } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'
import { EventsOn } from '../../wailsjs/runtime'
import type { RuntimeInfo } from '@/components/renderer/types'

interface StatusBarProps {
  runtimeStatus: Record<string, RuntimeInfo>
  runtimeLoading: boolean
}

const RUNTIME_ORDER = ['python', 'node', 'uv', 'bun']

const RUNTIME_LABELS: Record<string, string> = {
  python: 'Python',
  node: 'Node',
  uv: 'uv',
  bun: 'bun',
}

const ROW_H = 16

function RuntimeIcon({ info }: { info: RuntimeInfo }) {
  if (!info.available) {
    return <Minus className="h-3 w-3 text-muted-foreground/30 shrink-0" />
  }
  if (info.hint) {
    return <AlertTriangle className="h-3 w-3 text-amber-500 shrink-0" />
  }
  return <CheckCircle2 className="h-3 w-3 text-emerald-500 shrink-0" />
}

export function StatusBar({ runtimeStatus, runtimeLoading }: StatusBarProps) {
  const { t } = useI18n()
  const [phase, setPhase] = useState<'checking' | 'ready'>('checking')
  const [hintKey, setHintKey] = useState<string | null>(null)
  const [installMsg, setInstallMsg] = useState<string | null>(null)
  const hintTimer = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    if (!runtimeLoading && Object.keys(runtimeStatus).length > 0) {
      const timer = setTimeout(() => setPhase('ready'), 600)
      return () => clearTimeout(timer)
    }
  }, [runtimeLoading, runtimeStatus])

  useEffect(() => {
    const unsubProgress = EventsOn('runtime:install-progress', (data: { runtime: string; message: string; progress: number }) => {
      setInstallMsg(`${RUNTIME_LABELS[data.runtime] ?? data.runtime}: ${data.message} (${Math.round(data.progress)}%)`)
    })
    const unsubComplete = EventsOn('runtime:install-complete', (_data: { runtime: string; success: boolean }) => {
      setInstallMsg(null)
    })
    return () => {
      unsubProgress()
      unsubComplete()
    }
  }, [])

  const showHint = (key: string) => {
    clearTimeout(hintTimer.current)
    setHintKey(key)
    hintTimer.current = setTimeout(() => setHintKey(null), 8000)
  }

  const firstIssue = RUNTIME_ORDER.find(
    (key) => runtimeStatus[key]?.hint
  )

  return (
    <div className="flex h-6 shrink-0 items-center border-t border-border bg-card px-3 text-[11px] text-muted-foreground/70 select-none overflow-hidden">
      <div className="overflow-hidden relative" style={{ height: ROW_H }}>
        <div
          className="flex flex-col transition-transform duration-500"
          style={{ transform: `translateY(${phase === 'ready' ? `-${ROW_H}px` : '0'})` }}
        >
          {/* Checking row */}
          <div className="flex items-center gap-2" style={{ height: ROW_H }}>
            <Loader2 className="h-3 w-3 animate-spin text-muted-foreground/40 shrink-0" />
            <span className="whitespace-nowrap">{t('statusBar.checking')}</span>
            <span className="mx-1 text-muted-foreground/20">|</span>
            {RUNTIME_ORDER.map((key) => {
              const info = runtimeStatus[key]
              if (!info) return null
              return (
                <button
                  key={key}
                  className="flex items-center gap-0.5 hover:text-foreground transition-colors"
                  onClick={() => showHint(key)}
                  title={info.hint || ''}
                >
                  <RuntimeIcon info={info} />
                  <span className={
                    info.available
                      ? info.hint ? 'text-amber-600/80' : 'text-muted-foreground/70'
                      : 'text-muted-foreground/30'
                  }>
                    {RUNTIME_LABELS[key] ?? key}
                  </span>
                </button>
              )
            })}
          </div>
          {/* Ready row */}
          <div className="flex items-center gap-2" style={{ height: ROW_H }}>
            <span className="inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" />
            <span className="truncate">
              {installMsg ? (
                <span className="flex items-center gap-1 text-cyan-600/80">
                  <Loader2 className="h-3 w-3 animate-spin shrink-0" />
                  {installMsg}
                </span>
              ) : hintKey && runtimeStatus[hintKey]?.hint ? (
                `⚠ ${RUNTIME_LABELS[hintKey] ?? hintKey}: ${runtimeStatus[hintKey].hint!.split('\n')[0]}`
              ) : firstIssue ? (
                `⚠ ${RUNTIME_LABELS[firstIssue] ?? firstIssue} ${t('statusBar.hasIssues')}`
              ) : (
                t('statusBar.ready')
              )}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
