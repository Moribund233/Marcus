import { Store } from 'lucide-react'
import { useI18n } from '@/hooks/useI18n'

export function MarketTab() {
  const { t } = useI18n()
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 text-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted">
        <Store className="h-8 w-8 text-muted-foreground" />
      </div>
      <p className="text-base font-medium">{t('toolAdd.market.title')}</p>
      <p className="max-w-xs text-sm text-muted-foreground">
        {t('toolAdd.market.desc')}
      </p>
    </div>
  )
}
