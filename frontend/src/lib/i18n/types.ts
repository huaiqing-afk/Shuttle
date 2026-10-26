export type Language = 'zh-CN' | 'en-US'
export type LanguageChoice = 'auto' | Language

export interface I18nContextValue {
  language: Language
  choice: LanguageChoice
  setChoice: (choice: LanguageChoice) => void
  t: (key: string, params?: Record<string, string | number>) => string
}
