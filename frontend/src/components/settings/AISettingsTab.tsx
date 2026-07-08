import { useState, useEffect, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'
import { llm, model } from '../../../wailsjs/go/models'
import {
  GetLLMConfig,
  SaveLLMConfig,
  TestLLMConnection,
  GetLLMModels,
  GetSupportedProviders,
} from '../../../wailsjs/go/main/App'
import { Loader2, TestTube2, Save, CheckCircle2, AlertCircle } from 'lucide-react'

/**
 * AI 设置标签页。
 *
 * 用于配置 LLM Provider、API Key、模型与 Base URL，并提供连接测试与模型列表刷新功能。
 */
export function AISettingsTab() {
  const { t } = useI18n()
  const [config, setConfig] = useState<llm.Config>({
    provider: 'openai',
    api_key: '',
    model: '',
    base_url: '',
  })
  const [providers, setProviders] = useState<model.ProviderInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [modelsLoading, setModelsLoading] = useState(false)
  const [models, setModels] = useState<{ id: string; name: string }[]>([])
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  useEffect(() => {
    setLoading(true)
    Promise.all([GetLLMConfig(), GetSupportedProviders()])
      .then(([cfg, providerList]) => {
        if (providerList) {
          setProviders(providerList)
        }
        if (cfg) {
          setConfig({
            provider: cfg.provider || 'openai',
            api_key: cfg.api_key || '',
            model: cfg.model || '',
            base_url: cfg.base_url || '',
          })
        }
      })
      .catch((err) => console.error('load config failed', err))
      .finally(() => setLoading(false))
  }, [])

  const handleChange = useCallback(
    (field: keyof llm.Config, value: string) => {
      setConfig((prev) => ({ ...prev, [field]: value }))
      setStatus(null)
    },
    [],
  )

  const handleProviderChange = useCallback(
    (provider: string) => {
      const p = providers.find((x) => x.provider === provider)
      setConfig((prev) => ({
        ...prev,
        provider,
        model: p?.default_model || '',
        base_url: p?.default_base_url || '',
      }))
      setModels([])
      setStatus(null)
    },
    [providers],
  )

  const handleSave = useCallback(async () => {
    setSaving(true)
    setStatus(null)
    try {
      await SaveLLMConfig(config)
      setStatus({ type: 'success', message: t('aiSettings.saveSuccess') })
    } catch (e) {
      setStatus({ type: 'error', message: String(e) })
    } finally {
      setSaving(false)
    }
  }, [config, t])

  const handleTest = useCallback(async () => {
    setTesting(true)
    setStatus(null)
    try {
      await SaveLLMConfig(config)
      await TestLLMConnection()
      setStatus({ type: 'success', message: t('aiSettings.testSuccess') })
    } catch (e) {
      setStatus({ type: 'error', message: String(e) })
    } finally {
      setTesting(false)
    }
  }, [config, t])

  const handleLoadModels = useCallback(async () => {
    setModelsLoading(true)
    try {
      const list = await GetLLMModels()
      setModels(list.map((m) => ({ id: m.id, name: m.name || m.id })))
    } catch (e) {
      setStatus({ type: 'error', message: String(e) })
    } finally {
      setModelsLoading(false)
    }
  }, [])

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t('aiSettings.loading')}
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-2xl p-6">
      <div className="mb-6">
        <h2 className="text-base font-medium">{t('aiSettings.title')}</h2>
        <p className="text-sm text-muted-foreground">{t('aiSettings.desc')}</p>
      </div>

      <div className="space-y-5">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t('aiSettings.provider')}</label>
          <div className="flex flex-wrap gap-2">
            {providers.map((p) => (
              <button
                key={p.provider}
                onClick={() => handleProviderChange(p.provider)}
                className={`rounded-md border px-3 py-1.5 text-sm transition-colors ${
                  config.provider === p.provider
                    ? 'border-primary bg-primary/10 text-primary'
                    : 'border-input bg-background hover:bg-accent'
                }`}
              >
                {p.name}
              </button>
            ))}
          </div>
        </div>

        {providers.find((p) => p.provider === config.provider)?.need_api_key !== false && (
          <div className="space-y-2">
            <label className="text-sm font-medium">{t('aiSettings.apiKey')}</label>
            <input
              type="password"
              value={config.api_key}
              onChange={(e) => handleChange('api_key', e.target.value)}
              placeholder={t('aiSettings.apiKeyPlaceholder')}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            />
            <p className="text-xs text-muted-foreground">{t('aiSettings.apiKeyDesc')}</p>
          </div>
        )}

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">{t('aiSettings.model')}</label>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleLoadModels}
              disabled={modelsLoading}
              className="h-7 px-2 text-xs"
            >
              {modelsLoading ? (
                <Loader2 className="mr-1 h-3 w-3 animate-spin" />
              ) : (
                <Loader2 className="mr-1 h-3 w-3" />
              )}
              {t('aiSettings.loadModels')}
            </Button>
          </div>
          <select
            value={config.model}
            onChange={(e) => handleChange('model', e.target.value)}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="">{t('aiSettings.selectModel')}</option>
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t('aiSettings.baseUrl')}</label>
          <input
            type="text"
            value={config.base_url}
            onChange={(e) => handleChange('base_url', e.target.value)}
            placeholder={t('aiSettings.baseUrlPlaceholder')}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          />
          <p className="text-xs text-muted-foreground">{t('aiSettings.baseUrlDesc')}</p>
        </div>

        {status && (
          <div
            className={`flex items-center gap-2 rounded-md px-3 py-2 text-sm ${
              status.type === 'success'
                ? 'bg-green-500/10 text-green-600 dark:text-green-400'
                : 'bg-destructive/10 text-destructive'
            }`}
          >
            {status.type === 'success' ? (
              <CheckCircle2 className="h-4 w-4" />
            ) : (
              <AlertCircle className="h-4 w-4" />
            )}
            {status.message}
          </div>
        )}

        <div className="flex gap-3 pt-2">
          <Button onClick={handleSave} disabled={saving}>
            {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
            {t('aiSettings.save')}
          </Button>
          <Button variant="outline" onClick={handleTest} disabled={testing}>
            {testing ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <TestTube2 className="mr-2 h-4 w-4" />
            )}
            {t('aiSettings.testConnection')}
          </Button>
        </div>
      </div>
    </div>
  )
}
