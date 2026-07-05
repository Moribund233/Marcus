import { FolderOpen, FileUp } from 'lucide-react'
import { OpenFileDialog, OpenDirectoryDialog } from '../../../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import type { UIInput } from '@/components/renderer/types'

interface FormRendererProps {
  inputs: UIInput[]
  values: Record<string, string>
  onChange: (name: string, value: string) => void
}

function FileInput({ input, value, onChange }: { input: UIInput; value: string; onChange: (v: string) => void }) {
  const handleBrowse = async () => {
    const path = input.type === 'directory'
      ? await OpenDirectoryDialog()
      : await OpenFileDialog('*.*')
    if (path) onChange(path)
  }

  return (
    <div className="flex items-center gap-2">
      <input
        className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
        type="text"
        value={value}
        readOnly
        placeholder={input.type === 'directory' ? '选择目录...' : '选择文件...'}
      />
      <Button variant="outline" size="sm" onClick={handleBrowse}>
        {input.type === 'directory' ? <FolderOpen className="h-4 w-4" /> : <FileUp className="h-4 w-4" />}
        浏览
      </Button>
    </div>
  )
}

export function FormRenderer({ inputs, values, onChange }: FormRendererProps) {
  return (
    <div className="flex flex-col gap-4">
      {inputs.map((input) => (
        <div key={input.name} className="flex flex-col gap-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            {input.label}
            {input.required && <span className="ml-1 text-destructive">*</span>}
          </label>

          {input.type === 'select' && input.options ? (
            <select
              className="rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
              value={values[input.name] ?? String(input.default ?? '')}
              onChange={(e) => onChange(input.name, e.target.value)}
            >
              {input.options.map((opt) => (
                <option key={opt} value={opt}>{opt}</option>
              ))}
            </select>
          ) : input.type === 'file' || input.type === 'directory' ? (
            <FileInput
              input={input}
              value={values[input.name] ?? ''}
              onChange={(v) => onChange(input.name, v)}
            />
          ) : (
            <input
              className="rounded-lg border border-border bg-card px-3 py-2 text-sm outline-none transition-colors focus:border-primary/50"
              type={input.type === 'number' ? 'number' : 'text'}
              placeholder={input.placeholder}
              value={values[input.name] ?? String(input.default ?? '')}
              onChange={(e) => onChange(input.name, e.target.value)}
            />
          )}
        </div>
      ))}
    </div>
  )
}
