import { describe, expect, it } from 'vitest'
import { normalizeLanguage, parseStoredLanguage, parseStoredTheme } from './preferences'

describe('persisted preferences', () => {
  it('rejects unsupported or corrupt language values', () => {
    expect(parseStoredLanguage('zh-CN')).toBe('zh-CN')
    expect(parseStoredLanguage('en')).toBe('en')
    expect(parseStoredLanguage('fr')).toBeNull()
    expect(parseStoredLanguage('')).toBeNull()
  })

  it('normalizes browser and i18next language variants', () => {
    expect(normalizeLanguage('zh-Hans')).toBe('zh-CN')
    expect(normalizeLanguage('en-US')).toBe('en')
    expect(normalizeLanguage('fr')).toBe('en')
  })

  it('falls back safely for unsupported or corrupt themes', () => {
    expect(parseStoredTheme('light')).toBe('light')
    expect(parseStoredTheme('dark')).toBe('dark')
    expect(parseStoredTheme('system')).toBe('system')
    expect(parseStoredTheme('sepia')).toBe('system')
    expect(parseStoredTheme(null)).toBe('system')
  })
})
