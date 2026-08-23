import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import zhCN from './zh-CN.json'
import en from './en.json'
import { normalizeLanguage, parseStoredLanguage, type SupportedLanguage } from '@/lib/preferences'

export type Language = SupportedLanguage

function savedLanguage(): Language | null {
  if (typeof window === 'undefined') return null
  try {
    return parseStoredLanguage(window.localStorage.getItem('lang'))
  } catch {
    return null
  }
}

const savedLang = savedLanguage()
const browserLang = typeof navigator !== 'undefined' && navigator.language.startsWith('zh') ? 'zh-CN' : 'en'

i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
    en: { translation: en },
  },
  lng: savedLang || browserLang,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
})

function persistLanguage(language: string) {
  const normalized = normalizeLanguage(language)
  document.documentElement.lang = normalized
  try {
    window.localStorage.setItem('lang', normalized)
  } catch {
    // Storage may be unavailable in hardened browsers or test environments.
  }
}

if (typeof document !== 'undefined' && typeof window !== 'undefined') {
  persistLanguage(i18n.language)
  i18n.on('languageChanged', persistLanguage)
}

export default i18n
