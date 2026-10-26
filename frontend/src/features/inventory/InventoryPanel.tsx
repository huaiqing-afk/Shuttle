import { useEffect, useState } from 'react'
import { Button } from '../../components/ui/button'
import { errorMessage, type Bridge } from '../../lib/bridge'
import { InstallSection } from './InstallSection'
import { useI18n } from '../../lib/i18n'
import type { InstallResult, InventoryItem, InventoryPaths, InventoryReport, RemovalPreview } from './types'

// ownership names are localized dynamically in the component

// Only managed items may ever be selected for removal; the backend re-checks.
const removable = (item: InventoryItem) => item.Ownership === 'managed'

export function InventoryPanel({ bridge, onClose }: { bridge: Bridge | null; onClose: () => void }) {
  const { t } = useI18n()
  const [paths, setPaths] = useState<InventoryPaths | null>(null)
  const ownershipNames: Record<string, string> = {
    managed: t('inventory.ownership.managed'),
    external: t('inventory.ownership.external'),
    unknown: t('inventory.ownership.unknown'),
  }
  const [report, setReport] = useState<InventoryReport | null>(null)
  const [preview, setPreview] = useState<RemovalPreview | null>(null)
  const [result, setResult] = useState<InstallResult | null>(null)
  const [selected, setSelected] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) { event.stopImmediatePropagation(); onClose() } }
    document.addEventListener('keydown', onKey, true)
    return () => document.removeEventListener('keydown', onKey, true)
  }, [busy, onClose])

  const run = async (action: () => Promise<void>) => {
    setBusy(true); setError('')
    try { await action() } catch (reason) { setError(errorMessage(reason)) } finally { setBusy(false) }
  }
  const detect = () => run(async () => {
    if (!bridge) return
    setPreview(null); setResult(null); setSelected([])
    setReport(await bridge.app.DetectInventory())
  })
  const planRemoval = () => run(async () => {
    if (!bridge) return
    setPreview(null); setResult(null)
    setPreview(await bridge.app.PlanInventoryRemoval(selected))
  })
  const executeRemoval = () => {
    if (!bridge || !preview || preview.Plan.Blocked) return
    const token = preview.Token
    return run(async () => {
      setPreview(null); setResult(null)
      const res = await bridge.app.ExecuteInventoryRemoval(token)
      setResult(res)
      setReport(await bridge.app.DetectInventory())
      setSelected([])
    })
  }

  useEffect(() => {
    if (!bridge) return
    void run(async () => {
      setPaths(await bridge.app.InventoryPaths())
      setReport(await bridge.app.DetectInventory())
    })
  }, [bridge])

  const items = report?.Items ?? []
  const toggle = (name: string) => {
    setPreview(null); setResult(null)
    setSelected(current => current.includes(name) ? current.filter(item => item !== name) : [...current, name])
  }

  return <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" role="presentation">
    <section role="dialog" aria-modal="true" aria-labelledby="inventory-title"
      className="flex max-h-[90vh] w-full max-w-3xl flex-col rounded-xl border border-border bg-card text-foreground shadow-2xl">
      <header className="flex items-center justify-between gap-3 border-b border-border p-4">
        <h2 id="inventory-title" className="font-semibold">{t('inventory.title')}</h2>
        <Button variant="outline" disabled={busy} onClick={onClose} autoFocus>{t('inventory.close')}</Button>
      </header>
      <div className="space-y-5 overflow-y-auto p-5 text-sm" aria-busy={busy}>
        {!bridge && <p role="alert">{t('inventory.desktopOnly')}</p>}
        <p className="text-muted-foreground">{t('inventory.desc')}</p>
        {paths && <p className="break-all text-xs text-muted-foreground">{t('inventory.ledgerPath', { path: paths.Ledger })}</p>}
        <div className="flex flex-wrap items-center gap-3">
          <Button disabled={busy || !bridge} onClick={() => void detect()}>{t('inventory.btn.redetect')}</Button>
          <Button variant="outline" disabled={busy || selected.length === 0} onClick={() => void planRemoval()}>{t('inventory.btn.planRemoval')}</Button>
        </div>

        {error && <p role="alert" className="break-words text-destructive">{error}</p>}
        {report?.Note && <p className="rounded border border-border p-3 text-xs text-muted-foreground">{report.Note}</p>}

        <div className="space-y-2">
          <h3 className="font-medium">{t('inventory.sources.title')}</h3>
          {(report?.Sources ?? []).map(source => <p key={source.Name}
            className={source.Available ? 'break-all text-xs text-muted-foreground' : 'break-all text-xs text-destructive'}>
            {source.Name}：{source.Available ? t('inventory.sources.packages', { count: source.Count }) : t('inventory.sources.unavailable', { error: source.Error })}
          </p>)}
          {(report?.Sources ?? []).length === 0 && <p className="text-muted-foreground">{t('inventory.sources.empty')}</p>}
        </div>

        <InstallSection bridge={bridge} busy={busy} onBusy={setBusy} />

        <div className="space-y-2">
          <h3 className="font-medium">{t('inventory.items.title')}</h3>
          {items.length === 0 && <p className="text-muted-foreground">{t('inventory.items.empty')}</p>}
          {items.map(item => <article key={`${item.Source}:${item.Name}`} className="space-y-2 rounded border border-border p-3">
            <label className="flex flex-wrap items-start gap-3">
              <input type="checkbox" disabled={!removable(item) || busy} checked={selected.includes(item.Name)}
                aria-label={`选择 ${item.Name}`} onChange={() => toggle(item.Name)} className="mt-1" />
              <span className="min-w-0 flex-1 space-y-1">
                <span className="block break-all font-mono text-xs">{item.Name}</span>
                <span className="block text-xs text-muted-foreground">
                  {item.Version || t('inventory.items.versionUnknown')} · {item.Source} · {ownershipNames[item.Ownership] ?? item.Ownership}
                </span>
                <span className="block break-words text-xs text-muted-foreground">{item.Reason}</span>
              </span>
            </label>
            {!removable(item) && <p className="text-xs text-faint">{t('inventory.items.notRemovable')}</p>}
          </article>)}
        </div>

        {preview && <div className="space-y-3 rounded border border-border p-3">
          <h3 className="font-medium">{t('inventory.removalPlan.title')}</h3>
          {preview.Plan.Conflicts?.map(conflict => <p key={conflict} className="break-all text-xs text-destructive">{conflict}</p>)}
          {(preview.Plan.Steps ?? []).map(step => <div key={`${step.Item.Source}:${step.Item.Name}`} className="space-y-1">
            <p className={step.Action === 'uninstall' ? 'break-all text-xs' : 'break-all text-xs text-muted-foreground'}>
              {step.Item.Name}：{step.Action === 'uninstall' ? t('inventory.removalPlan.canUninstall') : t('inventory.removalPlan.skip', { reason: step.Item.Reason })}
            </p>
            {step.Action === 'uninstall' && step.Command && (
              <code className="block break-all rounded bg-hover p-2 font-mono text-[11px]">{step.Command}</code>
            )}
          </div>)}
          <Button disabled={busy || preview.Plan.Blocked || !(preview.Plan.Steps ?? []).some(s => s.Action === 'uninstall')}
            onClick={() => void executeRemoval()}>{t('inventory.removalPlan.btn.confirm')}</Button>
          {preview.Plan.Blocked && <p className="text-xs text-destructive">{t('inventory.removalPlan.blocked')}</p>}
          <p className="text-xs text-muted-foreground">{t('inventory.removalPlan.notice')}</p>
        </div>}

        {result && <div className="space-y-3 rounded border border-border p-3">
          <h4 className="text-xs font-medium">{t('inventory.removalPlan.resultTitle', { state: t(`inventory.state.${result.State}`) || result.State })}</h4>
          {result.Error && <p role="alert" className="break-words text-xs text-destructive">{result.Error}</p>}
          {(result.Records ?? []).map(record => <p key={`${record.Source}:${record.Name}`} className="break-all text-xs">
            {record.Name}：{!record.State ? t('inventory.removalPlan.removed') : (t(`inventory.state.${record.State}`) || record.State)}
            {record.InstallerError && <span className="text-destructive"> · {record.InstallerError}</span>}
          </p>)}
          <p className="text-[11px] text-muted-foreground">{t('inventory.install.opRecord', { id: result.OperationID })}</p>
        </div>}

      </div>
    </section>
  </div>
}
