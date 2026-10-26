import { useRef, useState } from 'react'
import { Button } from '../../components/ui/button'
import { useI18n } from '../../lib/i18n'
import { errorMessage, type Bridge } from '../../lib/bridge'
import type { InstallPreview, InstallResult } from './types'

// The component set is fixed by docs/architecture.md §2. Ids must match
// App.PlanInstall's table, which owns the real package identifiers.
const components = [
  { id: 'powershell', name: 'PowerShell 7', source: 'winget', url: 'https://learn.microsoft.com/powershell/scripting/install/installing-powershell-on-windows' },
  { id: 'git', name: 'Git', source: 'winget', url: 'https://git-scm.com/downloads/win' },
  { id: 'wt', name: 'Windows Terminal', source: 'winget', url: 'https://learn.microsoft.com/windows/terminal/install' },
  { id: 'node', name: 'Node.js (LTS)', source: 'winget', url: 'https://nodejs.org/en/download' },
  { id: 'claude', name: 'Claude Code', source: 'npm', url: 'https://code.claude.com/docs/en/setup' },
  { id: 'codex', name: 'Codex', source: 'npm', url: 'https://developers.openai.com/codex/cli/' },
]

const actionNames: Record<string, string> = { install: '安装', skip: '跳过' }
const stateNames: Record<string, string> = {
  succeeded: '成功', partially_failed: '部分失败', failed: '失败',
  installed: '已安装', unverified: '待核实', intent: '已记录安装意图',
}

// Commands come from the backend and are displayed before confirmation.
export function InstallSection({ bridge, busy, onBusy }: {
  bridge: Bridge | null; busy: boolean; onBusy: (busy: boolean) => void
}) {
  const { t } = useI18n()
  const [mode, setMode] = useState<'manual' | 'auto'>('manual')
  const [selected, setSelected] = useState<string[]>([])
  const [preview, setPreview] = useState<InstallPreview | null>(null)
  const [result, setResult] = useState<InstallResult | null>(null)
  const [error, setError] = useState('')
  const executing = useRef(false)
  const consumedToken = useRef<string | null>(null)

  const run = async (action: () => Promise<void>) => {
    onBusy(true); setError('')
    try { await action() } catch (reason) { setError(errorMessage(reason)) } finally { onBusy(false) }
  }
  const toggle = (id: string) => {
    setPreview(null); setResult(null)
    setSelected(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])
  }
  const plan = () => run(async () => {
    if (!bridge) return
    setPreview(null); setResult(null)
    setPreview(await bridge.app.PlanInstall(selected))
  })
  const execute = () => {
    if (!bridge || !preview || executing.current || consumedToken.current === preview.Token || mode !== 'auto') return
    const token = preview.Token
    consumedToken.current = token
    executing.current = true
    setPreview(null); setResult(null)
    return run(async () => {
      try { setResult(await bridge.app.ExecuteInstall(token)) }
      finally { executing.current = false }
    })
  }

  const steps = preview?.Plan.Steps ?? []
  return <div className="space-y-3">
    <h3 className="font-medium">{t('inventory.install.title')}</h3>
    <div className="flex flex-wrap items-center gap-3" role="group" aria-label={t('inventory.install.mode')}>
      <Button variant={mode === 'manual' ? 'default' : 'outline'} disabled={busy} aria-pressed={mode === 'manual'}
        onClick={() => { setMode('manual'); setPreview(null); setError('') }}>{t('inventory.install.manual')}</Button>
      <Button variant={mode === 'auto' ? 'default' : 'outline'} disabled={busy} aria-pressed={mode === 'auto'}
        onClick={() => { setMode('auto'); setPreview(null); setError('') }}>{t('inventory.install.auto')}</Button>
    </div>
    <p className="text-xs text-muted-foreground">
{mode === 'manual' ? t('inventory.install.manualDesc') : t('inventory.install.autoDesc')}
    </p>
    <p className="select-text break-all text-xs text-muted-foreground">{t('inventory.install.webview2Notice')}</p>
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {components.map(component => <label key={component.id} className="flex items-center gap-3 rounded border border-border p-2">
        <input type="checkbox" disabled={busy} checked={selected.includes(component.id)}
          aria-label={t('inventory.install.select', { name: component.name })} onChange={() => toggle(component.id)} />
        <span className="min-w-0 flex-1 text-xs">
          <span className="block truncate">{component.name}</span>
          <span className="block text-muted-foreground">{component.source}</span>
        </span>
      </label>)}
    </div>
    {mode === 'manual' && <div className="space-y-3">
      {components.filter(component => selected.includes(component.id)).map(component => <div key={component.id} className="space-y-1 rounded border border-border p-3">
        <p className="text-xs font-medium">{t('inventory.install.docTitle', { name: component.name })}</p>
        <p className="select-text break-all text-xs text-muted-foreground">{component.url}</p>
      </div>)}
      <p className="text-xs text-muted-foreground">{t('inventory.install.manualNotice')}</p>
    </div>}
    {mode === 'auto' && <Button disabled={busy || !bridge || selected.length === 0} onClick={() => void plan()}>{t('inventory.install.btn.preview')}</Button>}

    {error && <p role="alert" className="break-words text-xs text-destructive">{error}</p>}

    {mode === 'auto' && preview && <div className="space-y-3 rounded border border-border p-3">
      <h4 className="text-xs font-medium">{t('inventory.install.previewTitle')}</h4>
      {preview.Plan.Conflicts?.map(conflict => <p key={conflict} className="break-all text-xs text-destructive">{conflict}</p>)}
      {steps.map(step => <div key={`${step.Item.Source}:${step.Item.Name}`} className="space-y-1">
        <p className="break-all text-xs">
          {step.Item.Name}：{actionNames[step.Action] ?? step.Action}
          {step.Action === 'skip' && step.Item.Reason && <span className="text-muted-foreground"> · {step.Item.Reason}</span>}
        </p>
        {step.Action === 'install' && <>
          <code className="block break-all rounded bg-hover p-2 font-mono text-[11px]">{step.Command}</code>
          {step.Instruction && <p className="break-all text-[11px] text-muted-foreground">{step.Instruction}</p>}
        </>}
      </div>)}
      <Button disabled={busy || preview.Plan.Blocked || !steps.some(step => step.Action === 'install')}
        onClick={() => void execute()}>{t('inventory.install.btn.confirm')}</Button>
      {preview.Plan.Blocked && <p className="text-xs text-destructive">{t('inventory.install.conflict')}</p>}
    </div>}

    {result && <div className="space-y-3 rounded border border-border p-3">
      <h4 className="text-xs font-medium">{t('inventory.install.resultTitle', { state: t(`inventory.state.${result.State}`) || result.State })}</h4>
      {result.Error && <p role="alert" className="break-words text-xs text-destructive">{result.Error}</p>}
      {(result.Records ?? []).map(record => <p key={`${record.Source}:${record.Name}`} className="break-all text-xs">
        {record.Name}：{stateNames[record.State] ?? record.State}{record.Version ? ` · ${record.Version}` : ''}
        {record.InstallerError && <span className="text-destructive"> · {record.InstallerError}</span>}
      </p>)}
      <p className="text-[11px] text-muted-foreground">操作记录：{result.OperationID}</p>
    </div>}
  </div>
}
