import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import type { I18nContextValue, LanguageChoice } from './types.ts'
import { languageStorageKey, resolveLanguage, storedChoice, translate } from './core.ts'

const I18nContext = createContext<I18nContextValue>({
  language: 'zh-CN',
  choice: 'auto',
  setChoice: () => {},
  t: (key, params) => translate('zh-CN', key, params),
})

export function I18nProvider({ children }: { children: ReactNode }) {
  const [choice, setChoiceState] = useState<LanguageChoice>(storedChoice)
  const language = useMemo(() => resolveLanguage(choice), [choice])

  const setChoice = (newChoice: LanguageChoice) => {
    setChoiceState(newChoice)
    try {
      localStorage.setItem(languageStorageKey, newChoice)
    } catch {
      // private mode / quota exceeded
    }
  }

  const value = useMemo<I18nContextValue>(() => ({
    language,
    choice,
    setChoice,
    t: (key, params) => translate(language, key, params),
  }), [language, choice])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nContextValue {
  return useContext(I18nContext)
}
