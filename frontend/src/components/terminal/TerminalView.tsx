import { useEffect, useRef } from 'react'
import { Terminal } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'

interface TerminalViewProps {
  lines: string[]
}

export function TerminalView({ lines }: TerminalViewProps) {
  const { t } = useI18n()
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  return (
    <div className="overflow-hidden rounded-xl border border-border bg-[#0d1117]">
      <div className="flex items-center gap-2 border-b border-border/50 px-4 py-2">
        <Terminal className="h-3.5 w-3.5 text-muted-foreground/60" />
        <span className="text-[11px] font-medium text-muted-foreground/60">{t('terminal.output')}</span>
      </div>
      <div className="max-h-96 overflow-y-auto p-4 font-mono text-xs leading-relaxed text-[#e6edf3]">
        {lines.length === 0 ? (
          <span className="text-[#8b949e]">{t('terminal.waiting')}</span>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {line || '\u00A0'}
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
