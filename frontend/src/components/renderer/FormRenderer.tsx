import type { UIInput } from '@/components/renderer/types'

interface FormRendererProps {
  inputs: UIInput[]
  values: Record<string, string>
  onChange: (name: string, value: string) => void
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
