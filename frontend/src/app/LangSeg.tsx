import { Globe } from 'lucide-react'
import { useI18n, type LanguageChoice } from '../lib/i18n'

export function LangSeg() {
  const { setChoice, language } = useI18n()
  const next: LanguageChoice = language === 'zh-CN' ? 'en-US' : 'zh-CN'

  return <button
    id="app-language"
    aria-label="切换语言 / Switch Language"
    title={language === 'zh-CN' ? 'Switch to English' : '切换至中文'}
    onClick={() => setChoice(next)}
    className="flex h-7 items-center gap-1.5 rounded-md border border-border bg-background px-2 font-mono text-xs text-muted-foreground hover:bg-hover hover:text-foreground">
    <Globe className="size-3.5" aria-hidden="true" />
    <span>{language === 'zh-CN' ? 'EN' : '中'}</span>
  </button>
}
