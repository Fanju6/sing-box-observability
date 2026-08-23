export type SupportedLanguage = 'zh-CN' | 'en'
export type SupportedTheme = 'light' | 'dark' | 'system'

export function parseStoredLanguage(value: string | null): SupportedLanguage | null {
  return value === 'zh-CN' || value === 'en' ? value : null
}

export function normalizeLanguage(value: string): SupportedLanguage {
  return value.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en'
}

export function parseStoredTheme(value: string | null): SupportedTheme {
  return value === 'light' || value === 'dark' || value === 'system' ? value : 'system'
}
