import { Monitor, Moon, Sun } from 'lucide-react'
import { cn } from '../lib/utils'
import { useI18n } from '../lib/i18n'
import type { ThemeMode } from './theme'

const modes = [
  { id: 'system', key: 'theme.system', Icon: Monitor },
  { id: 'dark', key: 'theme.dark', Icon: Moon },
  { id: 'light', key: 'theme.light', Icon: Sun },
] as const

export function ThemeSeg({ mode, onChange }: { mode: ThemeMode; onChange: (mode: ThemeMode) => void }) {
  const { t } = useI18n()
  return <div role="radiogroup" aria-label={t('theme.group')} className="flex gap-0.5 rounded-lg border border-border bg-background p-0.5">
    {modes.map(({ id, key, Icon }) => {
      const label = t(key)
      return <button key={id} title={label} aria-label={label}
        aria-pressed={mode === id} onClick={() => onChange(id)}
        className={cn('grid size-[26px] place-items-center rounded-md text-faint hover:bg-hover hover:text-foreground',
          mode === id && 'bg-elevated text-primary')}>
        <Icon className="size-3.5" aria-hidden="true" />
      </button>
    })}
  </div>
}
