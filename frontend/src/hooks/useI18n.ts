import { createContext, useContext, useState, useCallback, createElement, type ReactNode } from 'react'
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

type I18nKey = keyof typeof zh

interface I18nContextValue {
  locale: string
  t: (key: I18nKey, params?: Record<string, string | number>) => string
  setLocale: (lang: string) => void
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState(getStoredLocale)

  const t = useCallback((key: I18nKey, params?: Record<string, string | number>) => {
    const msg = LOCALE_MAP[locale]?.[key]
    if (msg === undefined) return key
    if (!params) return msg
    return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
  }, [locale])

  const setLocale = useCallback((lang: string) => {
    localStorage.setItem(STORAGE_KEY, lang)
    setLocaleState(lang)
  }, [])

  return createElement(I18nContext.Provider, { value: { locale, t, setLocale } }, children)
}

export function useI18n() {
  const ctx = useContext(I18nContext)
  if (!ctx) {
    const locale = getStoredLocale()
    return {
      locale,
      t: (key: I18nKey, params?: Record<string, string | number>) => {
        const msg = LOCALE_MAP[locale]?.[key]
        if (msg === undefined) return key
        if (!params) return msg
        return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
      },
      setLocale: (lang: string) => { localStorage.setItem(STORAGE_KEY, lang) },
    }
  }
  return ctx
}
