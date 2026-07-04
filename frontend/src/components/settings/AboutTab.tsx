import { Package, Code2, ExternalLink } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'

export function AboutTab() {
  const { t } = useI18n()

  return (
    <div className="mx-auto max-w-lg p-6 pt-8">
      <div className="flex flex-col items-center gap-4 py-8">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10">
          <span className="text-2xl font-bold text-primary font-mono">M</span>
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
          href="https://gitee.com/xp_2133765105/marcus"
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
      </div>

      <p className="mt-8 text-center text-xs text-muted-foreground/40">
        {t('about.footer')}
      </p>
    </div>
  )
}
