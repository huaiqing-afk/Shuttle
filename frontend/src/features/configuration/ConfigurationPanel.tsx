import { useEffect, useState } from 'react'
import { Button } from '../../components/ui/button'
import { errorMessage, type Bridge } from '../../lib/bridge'
import type { ConfigChoice, ConfigKey, ConfigOperation, ConfigPaths, ConfigPlan } from './types'
import { ManagedProfileField, KeysField, ParamsField, baseFields } from './ConfigFields'
import { useI18n } from '../../lib/i18n'

const stateNames: Record<string, string> = { running: '执行中', interrupted: '上次操作中断', succeeded: '完成', failed: '失败', partially_failed: '部分失败' }

// "path = raw JSON" per line; values are passed through as JSON literals.
function parseParams(text: string): Record<string, string> {
 const out: Record<string, string> = {}
 for (const line of text.split('\n')) {
  const trimmed = line.trim()
  if (!trimmed || trimmed.startsWith('#')) continue
  const at = trimmed.indexOf('=')
  if (at < 1) continue
  out[trimmed.slice(0, at).trim()] = trimmed.slice(at + 1).trim()
 }
 return out
}

function parseParamsError(text: string): string {
 for (const line of text.split('\n')) {
  const trimmed = line.trim()
  if (!trimmed || trimmed.startsWith('#')) continue
  const at = trimmed.indexOf('=')
  if (at < 1) return `额外标量格式应为「路径 = JSON 值」：${trimmed}`
  const value = trimmed.slice(at + 1).trim()
  if (!value) return `额外标量缺少值：${trimmed}`
  try { JSON.parse(value) } catch { return `额外标量的值必须是合法 JSON：${trimmed}` }
 }
 return ''
}

function keysError(keys: ConfigKey[]): string {
 for (const [index, key] of keys.entries()) {
  if (!key.Keys.trim() || !key.Command.trim()) return `快捷键 ${index + 1} 需要同时填写按键与命令`
  try { JSON.parse(key.Value) } catch { return `快捷键 ${index + 1} 的值必须是合法 JSON` }
 }
 return ''
}

const emptyChoice: ConfigChoice = {
  SchemaVersion: 1,
  WT: { Theme: '', FontFace: '', DefaultProfileGUID: '', ManagedProfile: { GUID: '', Payload: '' }, Keys: [], Params: {} },
  PS: { SessionEntry: false },
}

export function ConfigurationPanel({ bridge, onClose }: { bridge: Bridge | null; onClose: () => void }) {
 const { t } = useI18n()
 const [paths, setPaths] = useState<ConfigPaths | null>(null)
 const [operations, setOperations] = useState<ConfigOperation[]>([])
 const [choice, setChoice] = useState<ConfigChoice>(emptyChoice)
 const [keys, setKeys] = useState<ConfigKey[]>([])
 const [params, setParams] = useState('')
 const [plan, setPlan] = useState<ConfigPlan | null>(null)
 const [busy, setBusy] = useState(false)
 const [error, setError] = useState('')
 const [message, setMessage] = useState('')
 // any edit invalidates the previewed plan, so a stale plan can never be applied
 const edit = <T,>(setter: (value: T) => void) => (value: T) => { setter(value); setPlan(null) }
 const update = (patch: Partial<ConfigChoice['WT']>) => edit(setChoice)({ ...choice, WT: { ...choice.WT, ...patch } })
 const refresh = async () => {
  if (!bridge) return
  const [p, ops] = await Promise.all([bridge.app.ConfigurationPaths(), bridge.app.ListConfigurationOperations()])
  setPaths(p); setOperations(ops ?? [])
 }
 useEffect(() => { void refresh().catch(reason => setError(errorMessage(reason))) }, [bridge])
 useEffect(() => {
  const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) { event.stopImmediatePropagation(); onClose() } }
  document.addEventListener('keydown', onKey, true)
  return () => document.removeEventListener('keydown', onKey, true)
 }, [busy, onClose])
 const run = async (action: () => Promise<void>) => {
  setBusy(true); setError(''); setMessage('')
  try { await action() } catch (reason) { setError(errorMessage(reason)) } finally { setBusy(false) }
 }
 const preview = () => run(async () => {
  if (!bridge) return
  setPlan(await bridge.app.PlanConfiguration({ ...choice, WT: { ...choice.WT, Keys: keys, Params: parseParams(params) } }))
 })
 const execute = () => run(async () => {
  if (!bridge || !plan) return
  const id = plan.PlanID; setPlan(null)
  const result = await bridge.app.ExecuteConfiguration(id)
  setMessage(result.Succeeded ? t('config.msg.completed') : t('config.msg.partial'))
  await refresh()
 })
 const configured = choice.WT.Theme !== '' || choice.WT.FontFace !== '' || choice.WT.DefaultProfileGUID !== '' ||
   choice.WT.ManagedProfile.GUID !== '' || keys.length > 0 || params.trim() !== '' || choice.PS.SessionEntry
 return <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" role="presentation">
  <section role="dialog" aria-modal="true" aria-labelledby="configuration-title" className="flex max-h-[90vh] w-full max-w-3xl flex-col rounded-xl border border-border bg-card text-foreground shadow-2xl">
   <header className="flex items-center justify-between gap-3 border-b border-border p-4">
    <h2 id="configuration-title" className="font-semibold">{t('config.title')}</h2>
    <Button variant="outline" disabled={busy} onClick={onClose} autoFocus>{t('config.close')}</Button>
   </header>
   <div className="space-y-5 overflow-y-auto p-5 text-sm" aria-busy={busy}>
    {!bridge && <p role="alert">{t('config.desktopOnly')}</p>}
    <p className="text-muted-foreground">{t('config.desc')}</p>
    {paths && <div className="space-y-2 break-all text-xs text-muted-foreground"><p>{t('config.paths.wt', { path: paths.WT })}</p><p>{t('config.paths.ps', { path: paths.PowerShell })}</p><p>{t('config.paths.ops', { path: paths.Operations })}</p></div>}
    <fieldset disabled={busy || !bridge || !paths} className="space-y-4">
     {baseFields(choice, update)}
     <ManagedProfileField value={choice.WT.ManagedProfile} onChange={next => update({ ManagedProfile: next })} />
     <KeysField value={keys} onChange={next => edit(setKeys)(next)} />
     <ParamsField value={params} onChange={next => edit(setParams)(next)} />
     <label className="flex flex-wrap items-center gap-3"><input type="checkbox" checked={choice.PS.SessionEntry}
       onChange={event => edit(setChoice)({ ...choice, PS: { SessionEntry: event.target.checked } })} />启用 PowerShell 历史预测</label>
     <Button disabled={!configured || Boolean(parseParamsError(params)) || Boolean(keysError(keys))} onClick={() => void preview()}>预览差异</Button>
    </fieldset>
    {parseParamsError(params) && <p role="alert" className="text-destructive">{parseParamsError(params)}</p>}
    {keysError(keys) && <p role="alert" className="text-destructive">{keysError(keys)}</p>}
    {error && <p role="alert" className="break-words text-destructive">{error}</p>}
    {message && <p role="status">{message}</p>}
    {plan && <div className="space-y-3">
     <h3 className="font-medium">{t('config.diff.title')}</h3>
     {(plan.Diffs ?? []).map(diff => <article key={diff.Target} className="space-y-2 rounded border border-border p-3">
      <p className="break-all font-mono text-xs">{diff.Target} · {diff.Action === 'none' ? t('config.diff.noChange') : diff.Action}</p>
      <div className="grid gap-3 sm:grid-cols-2"><pre className="whitespace-pre-wrap break-all rounded bg-background p-2 text-xs">{t('config.diff.original')}{diff.WasAbsent ? t('config.diff.notPresent') : diff.Before}</pre><pre className="whitespace-pre-wrap break-all rounded bg-background p-2 text-xs">{t('config.diff.target')}{diff.After || t('config.diff.removeManaged')}</pre></div>
      {diff.Conflict && <p className="text-destructive">{diff.Conflict}</p>}
     </article>)}
     <Button disabled={busy || plan.Blocked || !(plan.Diffs ?? []).some(diff => diff.Action !== 'none')} onClick={() => void execute()}>{t('config.btn.apply')}</Button>
    </div>}
    <div className="space-y-3"><h3 className="font-medium">操作记录</h3>
     {operations.length === 0 && <p className="text-muted-foreground">尚无配置操作。</p>}
     {operations.map(op => <article key={op.ID} className="space-y-3 rounded border border-border p-3">
      <p className="break-all">{stateNames[op.State] ?? op.State} · {new Date(op.StartedAt).toLocaleString()}</p>
      <p className="break-all font-mono text-xs text-muted-foreground">{op.ID}</p>
      {(op.Steps ?? []).map(step => <p key={step.Target} className={step.Err ? 'break-all text-destructive' : 'break-all text-xs text-muted-foreground'}>{step.Target} · {step.Err || (step.Action === 'pending' ? t('config.ops.pendingVerify') : step.Action)}</p>)}
      <Button variant="outline" disabled={busy} onClick={() => void run(async () => { if (bridge) setPlan(await bridge.app.PlanConfigurationRestore(op.ID)) })}>{t('config.btn.restorePreview')}</Button>
     </article>)}
    </div>
   </div>
  </section>
 </div>
}
