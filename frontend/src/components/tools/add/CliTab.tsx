import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'

interface CliTabProps {
  onAdd: (name: string, command: string, argType: string) => void
  onCancel: () => void
}

export function CliTab({ onAdd, onCancel }: CliTabProps) {
  const { t } = useI18n()
  const [name, setName] = useState('')
  const [command, setCommand] = useState('')
  const [argType, setArgType] = useState('directory')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name || !command) return
    onAdd(name, command, argType)
  }

  return (
    <div className="mx-auto w-full max-w-lg pt-8">
      <form onSubmit={handleSubmit} className="flex flex-col gap-5">
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium text-muted-foreground">{t('toolAddManual.name')}</label>
          <input
            className="rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
            placeholder={t('toolAddManual.namePlaceholder')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium text-muted-foreground">{t('toolAddManual.command')}</label>
          <input
            className="font-mono rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
            placeholder={t('toolAddManual.commandPlaceholder')}
            value={command}
            onChange={(e) => setCommand(e.target.value)}
          />
          <p className="text-[11px] text-muted-foreground/70">
            {t('toolAddManual.commandHint')}
          </p>
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium text-muted-foreground">{t('toolAddManual.argType')}</label>
          <select
            className="rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
            value={argType}
            onChange={(e) => setArgType(e.target.value)}
          >
            <option value="directory">{t('toolAddManual.directory')}</option>
            <option value="file">{t('toolAddManual.file')}</option>
            <option value="string">{t('toolAddManual.text')}</option>
          </select>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={!name || !command}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t('toolAddManual.add')}
          </Button>
          <Button type="button" variant="ghost" onClick={onCancel}>
            <X className="mr-1.5 h-4 w-4" />
            {t('toolAddManual.cancel')}
          </Button>
        </div>
      </form>
    </div>
  )
}
