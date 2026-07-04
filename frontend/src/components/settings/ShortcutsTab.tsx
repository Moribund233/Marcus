import { Keyboard, Command, ArrowBigUp } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'

export function ShortcutsTab() {
  const { t } = useI18n()

  const SHORTCUTS = [
    { keys: ['Ctrl', 'K'], desc: t('shortcuts.palette') },
    { keys: ['Ctrl', 'N'], desc: t('shortcuts.addTool') },
    { keys: ['Ctrl', 'R'], desc: t('shortcuts.refresh') },
    { keys: ['Ctrl', 'E'], desc: t('shortcuts.settings') },
    { keys: ['Ctrl', 'W'], desc: t('shortcuts.back') },
  ] as const

  return (
    <div className="mx-auto max-w-lg p-6 pt-8">
      <div className="flex items-center gap-2">
        <Keyboard className="h-4 w-4 text-muted-foreground" />
        <h2 className="text-base font-medium">{t('shortcuts.title')}</h2>
      </div>
      <p className="mt-1 text-sm text-muted-foreground">{t('shortcuts.desc')}</p>

      <div className="mt-8 flex flex-col gap-2">
        {SHORTCUTS.map((s, i) => (
          <div
            key={i}
            className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3"
          >
            <span className="text-sm">{s.desc}</span>
            <div className="flex items-center gap-1">
              {s.keys.map((k, j) => (
                <span key={j}>
                  <kbd className="inline-flex h-6 min-w-6 items-center justify-center rounded border border-border bg-background px-1.5 text-[11px] font-medium text-foreground/80">
                    {k}
                  </kbd>
                  {j < s.keys.length - 1 && <span className="mx-0.5 text-xs text-muted-foreground">+</span>}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
