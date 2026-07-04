import { useState, useCallback } from 'react'
import zh from '@/locales/zh-CN'
import en from '@/locales/en-US'

const STORAGE_KEY = 'marcus_language'

const LOCALE_MAP: Record<string, Record<string, string>> = {
  'zh-CN': zh,
  'en-US': en,
}

function getStoredLocale(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) || 'zh-CN'
  } catch {
    return 'zh-CN'
  }
}

export function useI18n() {
  const [locale, setLocaleState] = useState(getStoredLocale)

  const t = useCallback((key: string, params?: Record<string, string | number>) => {
    const msg = LOCALE_MAP[locale]?.[key]
    if (msg === undefined) return key
    if (!params) return msg
    return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
  }, [locale])

  const setLocale = useCallback((lang: string) => {
    localStorage.setItem(STORAGE_KEY, lang)
    setLocaleState(lang)
  }, [])

  return { t, locale, setLocale }
}
