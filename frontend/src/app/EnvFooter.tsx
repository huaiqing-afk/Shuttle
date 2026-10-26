import { useEffect, useRef, useState } from 'react'
import { Check, ChevronRight, CircleHelp, RefreshCw } from 'lucide-react'
import { cn } from '../lib/utils'
import { errorMessage, getBridge, type Detection } from '../lib/bridge'
import { useI18n } from '../lib/i18n'

export function EnvFooter({ bridge }: { bridge: ReturnType<typeof getBridge> }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<Detection[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const busyRef = useRef(false)

  const detect = async () => {
    if (!bridge || busyRef.current) return
    busyRef.current = true
    setBusy(true)
    setError('')
    try {
      const result = await bridge.app.DetectEnvironment()
      if (!Array.isArray(result) || !result.every(item => item && typeof item.name === 'string' &&
        typeof item.path === 'string' && typeof item.available === 'boolean')) {
        throw new Error(t('env.formatError'))
      }
      setItems(result)
    } catch (reason) { setError(t('env.failed', { error: errorMessage(reason) })) }
    finally { busyRef.current = false; setBusy(false) }
  }

  useEffect(() => { void detect() }, [bridge])

  return <div className="shrink-0 border-t border-border px-3 py-2.5">
    <button aria-expanded={open} onClick={() => setOpen(value => !value)}
      className="flex w-full items-center gap-2.5 rounded-md px-1.5 py-1.5 font-mono text-[11px] text-muted-foreground hover:bg-hover hover:text-foreground">
      <span aria-hidden="true" className="flex gap-1.5">
        {[0, 1, 2].map(index => <i key={index}
          className={cn('size-1.5 rounded-full', items === null ? 'bg-faint opacity-40'
            : items[index]?.available ? 'bg-success' : 'bg-faint opacity-40')} />)}
      </span>
      <span>{t('env.title')}</span>
      <button aria-label={t('env.refresh')} disabled={!bridge || busy} onClick={() => void detect()}
        className="ml-auto grid size-5 place-items-center rounded text-faint hover:bg-hover hover:text-foreground disabled:opacity-40">
        <RefreshCw aria-hidden="true" className={cn('size-3', busy && 'animate-spin motion-reduce:animate-none')} />
      </button>
      <ChevronRight aria-hidden="true" className={cn('size-3 transition-transform', open && 'rotate-90')} />
    </button>
    {open && <div className="mt-1.5 flex flex-col gap-2 px-1.5 pb-1 text-[11px] leading-5">
      {items === null ? <p className="text-faint">{bridge ? t('env.detecting') : t('env.unavailable')}</p> :
        items.map((item, index) => <div key={`${item.name}-${index}`} className="min-w-0">
          <p className="flex items-center gap-1.5 text-muted-foreground">
            {item.available ? <Check aria-hidden="true" className="size-3 shrink-0 text-success" />
              : <CircleHelp aria-hidden="true" className="size-3 shrink-0 text-faint" />}
            <span className="font-mono">{item.name}</span>
            <span className={item.available ? 'text-success' : 'text-faint'}>{item.available ? t('env.available') : t('env.notAvailable')}</span>
          </p>
          <p className="break-all pl-4 font-mono text-[10px] leading-4 text-faint">{item.path || t('env.noPath')}</p>
        </div>)}
      {error && <p role="alert" className="break-words text-destructive">{error}</p>}
    </div>}
  </div>
}
