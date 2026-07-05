import type { UIOutputField } from '@/components/renderer/types'
import { useI18n } from '@/hooks/useI18n'

interface OutputRendererProps {
  fields: UIOutputField[]
  data?: Record<string, unknown>
}

export function OutputRenderer({ fields, data }: OutputRendererProps) {
  const { t } = useI18n()
  if (!fields.length) return null

  return (
    <div className="flex flex-col gap-3">
      <h3 className="text-sm font-medium">{t('outputRenderer.title')}</h3>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {fields.map((field) => (
          <div
            key={field.name}
            className="rounded-lg border border-border bg-card p-4"
          >
            <div className="text-xs text-muted-foreground">{field.label}</div>
            <div className="mt-1 text-sm">
              {data?.[field.name] !== undefined
                ? String(data[field.name])
                : <span className="text-muted-foreground/50">{t('outputRenderer.waiting')}</span>
              }
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
