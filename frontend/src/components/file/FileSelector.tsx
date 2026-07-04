import { FileUp, FolderOpen } from 'lucide-react'
import { OpenFileDialog, OpenDirectoryDialog } from '../../../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'

interface FileSelectorProps {
  inputType: string
  extensions?: string[]
  onSelect: (paths: string[]) => void
}

export function FileSelector({ inputType, extensions, onSelect }: FileSelectorProps) {
  const { t } = useI18n()
  const isDirectory = inputType === 'directory'

  const handleClick = async () => {
    const path = isDirectory
      ? await OpenDirectoryDialog()
      : await OpenFileDialog(extensions?.join(';') ?? '*.*')
    if (path) {
      onSelect([path])
    }
  }

  return (
    <div className="rounded-xl border-2 border-dashed border-border bg-card/50 p-8 text-center transition-colors hover:border-primary/30">
      <div className="mb-3 flex justify-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
          {isDirectory ? <FolderOpen className="h-6 w-6" /> : <FileUp className="h-6 w-6" />}
        </div>
      </div>
      <p className="text-sm font-medium">
        {isDirectory ? t('file.selectDir') : t('file.selectFile')}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">
        {extensions && extensions.length > 0
          ? t('file.supportedFormats', { formats: extensions.join(', ') })
          : isDirectory ? t('file.selectDirHint') : t('file.selectFileHint')
        }
      </p>
      <Button variant="outline" size="sm" className="mt-4" onClick={handleClick}>
        {t('file.browse')}
      </Button>
    </div>
  )
}
