import type { Language, LanguageChoice } from './types.ts'
import { zh, type TranslationKey } from './zh.ts'
import { en } from './en.ts'

export * from './types.ts'
export { zh } from './zh.ts'
export { en } from './en.ts'

export const languageStorageKey = 'shuttle-language'

export const dictionaries: Record<Language, Record<TranslationKey, string>> = {
  'zh-CN': zh,
  'en-US': en,
}

export const languageChoices: { id: LanguageChoice; labelKey: TranslationKey }[] = [
  { id: 'auto', labelKey: 'lang.auto' },
  { id: 'zh-CN', labelKey: 'lang.zh' },
  { id: 'en-US', labelKey: 'lang.en' },
]

export function normalizeChoice(value: string | null): LanguageChoice {
  if (value === 'zh-CN' || value === 'en-US') return value
  return 'auto'
}

export function resolveLanguage(choice: LanguageChoice, navLang?: string): Language {
  if (choice === 'zh-CN' || choice === 'en-US') return choice
  const browserLang = (navLang ?? (typeof navigator !== 'undefined' ? navigator.language : '')).toLowerCase()
  if (browserLang.startsWith('en')) return 'en-US'
  return 'zh-CN'
}

export function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (_, key) => {
    const val = params[key]
    return val !== undefined ? String(val) : `{${key}}`
  })
}

export function translate(lang: Language, key: string, params?: Record<string, string | number>): string {
  const dict = dictionaries[lang] ?? zh
  const template = (dict as Record<string, string>)[key] ?? (zh as Record<string, string>)[key] ?? key
  return interpolate(template, params)
}

export function storedChoice(): LanguageChoice {
  try {
    return normalizeChoice(localStorage.getItem(languageStorageKey))
  } catch {
    return 'auto'
  }
}
