import { Button } from '../../components/ui/button'
import { useI18n } from '../../lib/i18n'
import type { ConfigChoice, ConfigKey } from './types'

type WT = ConfigChoice['WT']
const input = 'rounded border border-border bg-background p-2 font-mono text-xs'

export function BaseFields({ choice, update }: { choice: ConfigChoice; update: (patch: Partial<WT>) => void }) {
  const { t } = useI18n()
  return <>
    <label className="flex flex-wrap items-center gap-3">{t('config.wtTheme')}
      <select className={input} value={choice.WT.Theme} onChange={event => update({ Theme: event.target.value })}>
        <option value="">{t('config.wtTheme.keep')}</option>
        <option value="system">{t('config.wtTheme.system')}</option>
        <option value="dark">{t('config.wtTheme.dark')}</option>
        <option value="light">{t('config.wtTheme.light')}</option>
      </select>
    </label>
    <label className="flex flex-wrap items-center gap-3">{t('config.fontFace')}
      <input className={input} value={choice.WT.FontFace} placeholder={t('config.fontFace.placeholder')} onChange={event => update({ FontFace: event.target.value })} />
    </label>
    <label className="flex flex-wrap items-center gap-3">{t('config.defaultGuid')}
      <input className={`${input} min-w-0 flex-1`} value={choice.WT.DefaultProfileGUID} placeholder="{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}"
        onChange={event => update({ DefaultProfileGUID: event.target.value })} />
    </label>
  </>
}

export function ManagedProfileField({ value, onChange }: { value: WT['ManagedProfile']; onChange: (next: WT['ManagedProfile']) => void }) {
  const { t } = useI18n()
  return <fieldset className="space-y-3 rounded border border-border p-3">
    <legend className="px-1 text-xs text-muted-foreground">{t('config.managedProfile')}</legend>
    <label className="flex flex-wrap items-center gap-3">GUID
      <input className={`${input} min-w-0 flex-1`} value={value.GUID} onChange={event => onChange({ ...value, GUID: event.target.value })} />
    </label>
    <label className="flex flex-col gap-2">Profile JSON
      <textarea className={`${input} min-h-24`} value={value.Payload} placeholder='{"guid":"{...}","name":"Shuttle","hidden":false}'
        onChange={event => onChange({ ...value, Payload: event.target.value })} />
    </label>
  </fieldset>
}

export function KeysField({ value, onChange }: { value: ConfigKey[]; onChange: (next: ConfigKey[]) => void }) {
  const { t } = useI18n()
  const replace = (index: number, patch: Partial<ConfigKey>) => onChange(value.map((item, i) => i === index ? { ...item, ...patch } : item))
  return <fieldset className="space-y-3 rounded border border-border p-3">
    <legend className="px-1 text-xs text-muted-foreground">{t('config.shortcuts')}</legend>
    {value.map((key, index) => <div key={index} className="flex flex-wrap items-center gap-3">
      <input aria-label={`${t('config.shortcuts')} ${index + 1} ${t('config.shortcuts.keys')}`} className={`${input} w-36`} value={key.Keys} onChange={event => replace(index, { Keys: event.target.value })} />
      <input aria-label={`${t('config.shortcuts')} ${index + 1} ${t('config.shortcuts.command')}`} className={`${input} w-36`} value={key.Command} onChange={event => replace(index, { Command: event.target.value })} />
      <input aria-label={`${t('config.shortcuts')} ${index + 1} ${t('config.shortcuts.value')}`} className={`${input} min-w-0 flex-1`} value={key.Value} onChange={event => replace(index, { Value: event.target.value })} />
      <Button variant="outline" aria-label={`${t('config.shortcuts.remove')} ${index + 1}`} onClick={() => onChange(value.filter((_, i) => i !== index))}>{t('config.shortcuts.remove')}</Button>
    </div>)}
    <Button variant="outline" onClick={() => onChange([...value, { Keys: '', Command: '', Value: '' }])}>{t('config.shortcuts.add')}</Button>
  </fieldset>
}

export function ParamsField({ value, onChange }: { value: string; onChange: (next: string) => void }) {
  const { t } = useI18n()
  return <label className="flex flex-col gap-2">{t('config.extraScalars')}
    <textarea className={`${input} min-h-20`} value={value}
      placeholder={'profiles.defaults.fontSize = 13\nprofiles.defaults.colorScheme = "One Half Dark"'}
      onChange={event => onChange(event.target.value)} />
  </label>
}

export function baseFields(choice: ConfigChoice, update: (patch: Partial<WT>) => void) {
  return <BaseFields choice={choice} update={update} />
}
