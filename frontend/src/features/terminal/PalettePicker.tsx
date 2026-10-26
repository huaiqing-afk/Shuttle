import { useI18n } from '../../lib/i18n'
import { terminalPalettes } from './palettes'

export function PalettePicker({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useI18n()
  return <div className="flex h-10 shrink-0 items-center justify-end gap-3 border-b border-border bg-card px-3">
    <label htmlFor="terminal-palette" className="text-xs text-muted-foreground">{t('terminal.palette.label')}</label>
    <select id="terminal-palette" value={value} onChange={event => onChange(event.target.value)}
      className="h-7 max-w-[min(220px,55vw)] rounded-md border border-border bg-card px-2 text-xs text-foreground focus-visible:outline-2 focus-visible:outline-primary">
      <option value="auto">{t('terminal.palette.auto')}</option>
      {(['dark', 'light'] as const).map(kind => <optgroup key={kind} label={kind === 'dark' ? t('terminal.palette.darkGroup') : t('terminal.palette.lightGroup')}>
        {terminalPalettes.filter(palette => palette.kind === kind).map(palette =>
          <option key={palette.id} value={palette.id}>{palette.name}</option>)}
      </optgroup>)}
    </select>
  </div>
}
