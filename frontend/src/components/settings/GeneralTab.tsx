import { Sun, Languages, Scan, LogOut, Moon, Palette } from 'lucide-react'
import { useConfig } from '@/hooks/useConfig'
import { useI18n } from '@/hooks/useI18n'

function TabSwitch({ options, value, onChange }: { options: { label: string; value: string }[]; value: string; onChange: (v: string) => void }) {
  const activeIndex = options.findIndex((o) => o.value === value)

  return (
    <div className="relative flex w-fit rounded-lg border border-border bg-card p-0.5">
      <div
        className="absolute top-0.5 bottom-0.5 rounded-md bg-accent transition-all duration-200"
        style={{
          left: `calc(${activeIndex} / ${options.length} * 100% + 2px)`,
          width: `calc(100% / ${options.length} - 4px)`,
        }}
      />
      {options.map((opt, i) => (
        <button
          key={opt.value}
          onClick={() => onChange(opt.value)}
          className={`relative z-10 whitespace-nowrap rounded-md px-4 py-1.5 text-xs font-medium transition-colors ${value === opt.value ? 'text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}

export function GeneralTab() {
  const { config, save } = useConfig()
  const { t, setLocale } = useI18n()
  const autoScan = config?.auto_scan ?? true
  const terminateOnExit = config?.terminate_on_exit ?? true
  const theme = config?.theme ?? 'dark'
  const lang = config?.language ?? 'zh-CN'

  const setTheme = (next: string) => {
    save({ theme: next })
    document.documentElement.classList.remove('dark', 'theme-marcus')
    if (next === 'dark') document.documentElement.classList.add('dark')
    if (next === 'marcus') document.documentElement.classList.add('theme-marcus')
  }

  return (
    <div className="mx-auto max-w-lg p-6 pt-8">
      <h2 className="text-base font-medium">{t('general.title')}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{t('general.desc')}</p>

      <div className="mt-8 flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {theme === 'dark' ? <Moon className="h-4 w-4 text-muted-foreground" /> : theme === 'marcus' ? <Palette className="h-4 w-4 text-muted-foreground" /> : <Sun className="h-4 w-4 text-muted-foreground" />}
            <div>
              <div className="text-sm">{t('general.theme')}</div>
              <div className="text-xs text-muted-foreground">{t('general.themeDesc')}</div>
            </div>
          </div>
          <TabSwitch
            options={[
              { label: t('general.themeDark'), value: 'dark' },
              { label: t('general.themeLight'), value: 'light' },
              { label: 'Marcus', value: 'marcus' },
            ]}
            value={theme}
            onChange={setTheme}
          />
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Languages className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('general.language')}</div>
              <div className="text-xs text-muted-foreground">{t('general.languageDesc')}</div>
            </div>
          </div>
          <TabSwitch
            options={[
              { label: t('general.langZh'), value: 'zh-CN' },
              { label: t('general.langEn'), value: 'en-US' },
            ]}
            value={lang}
            onChange={(v) => { save({ language: v }); setLocale(v) }}
          />
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Scan className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('general.autoScan')}</div>
              <div className="text-xs text-muted-foreground">{t('general.autoScanDesc')}</div>
            </div>
          </div>
          <button
            onClick={() => save({ auto_scan: !autoScan })}
            className={`relative h-5 w-9 rounded-full p-0.5 transition-colors ${autoScan ? 'bg-primary' : 'bg-primary/30'}`}
          >
            <div className={`h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${autoScan ? 'translate-x-4' : 'translate-x-0'}`} />
          </button>
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <LogOut className="h-4 w-4 text-muted-foreground" />
            <div>
              <div className="text-sm">{t('general.terminateOnExit')}</div>
              <div className="text-xs text-muted-foreground">{t('general.terminateOnExitDesc')}</div>
            </div>
          </div>
          <button
            onClick={() => save({ terminate_on_exit: !terminateOnExit })}
            className={`relative h-5 w-9 rounded-full p-0.5 transition-colors ${terminateOnExit ? 'bg-primary' : 'bg-primary/30'}`}
          >
            <div className={`h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${terminateOnExit ? 'translate-x-4' : 'translate-x-0'}`} />
          </button>
        </div>
      </div>
    </div>
  )
}
