import { useI18n } from '@/hooks/useI18n'

interface ProgressViewProps {
  progress: number
}

export function ProgressView({ progress }: ProgressViewProps) {
  const { t } = useI18n()
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{t('file.processing')}</span>
        <span>{Math.round(progress)}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-secondary">
        <div
          className="h-full rounded-full bg-primary transition-all duration-300 ease-out"
          style={{ width: `${Math.min(progress, 100)}%` }}
        />
      </div>
    </div>
  )
}
