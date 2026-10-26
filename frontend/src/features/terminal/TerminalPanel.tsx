import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import type { ITheme } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Minus, Plus } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import { errorMessage, type Agent, type Bridge } from '../../lib/bridge'
import { TerminalSession, type Phase } from './session'
import type { Tab } from './types'
import { agentNames, phaseLabels } from './types'

import { PalettePicker } from './PalettePicker'
import { useI18n } from '../../lib/i18n'
import { normalizePalette, resolvePalette, terminalPaletteKey } from './palettes'

function storedPalette(): string {
  try { return normalizePalette(localStorage.getItem(terminalPaletteKey)) } catch { return 'auto' }
}
const monoFont = '"Cascadia Code", "Maple Mono NF CN", Consolas, monospace'

function TerminalView({ bridge, tab, active, fontSize, theme, onPhase, onDimensions }: {
  bridge: Bridge
  tab: Tab
  active: boolean
  fontSize: number
  theme: ITheme
  onPhase: (key: number, phase: Phase) => void
  onDimensions: (text: string) => void
}) {
  const host = useRef<HTMLDivElement>(null)
  const [error, setError] = useState('')
  const phaseRef = useRef(onPhase)
  const dimsRef = useRef(onDimensions)
  const activeRef = useRef(active)
  const themeRef = useRef(theme)
  const fit = useRef<() => void>(() => {})
  const term = useRef<Terminal | null>(null)
  phaseRef.current = onPhase
  dimsRef.current = onDimensions
  activeRef.current = active
  themeRef.current = theme

  useEffect(() => {
    if (!host.current) return
    let disposed = false
    let frame = 0
    const instance = new Terminal({
      cursorBlink: true, fontSize: 14, scrollback: 5000,
      fontFamily: monoFont, theme: themeRef.current, allowProposedApi: false,
    })
    const addon = new FitAddon()
    instance.loadAddon(addon)
    instance.open(host.current)
    const controller = new TerminalSession(bridge, instance,
      phase => { if (!disposed) phaseRef.current(tab.key, phase) },
      message => { if (!disposed) setError(message); else console.error(message) })
    term.current = instance
    // repeated same-size resizes each raise a ConPTY resize (SIGWINCH) that makes TUI
    // programs reprint their whole screen; dedupe by cols×rows signature
    let lastSent = ''
    const doFit = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        if (disposed || !host.current?.clientWidth || !host.current?.clientHeight) return
        try {
          addon.fit()
          const signature = `${instance.cols}x${instance.rows}`
          if (signature === lastSent) return
          lastSent = signature
          if (activeRef.current) dimsRef.current(`${instance.cols} × ${instance.rows}`)
          void controller.resize(instance.cols, instance.rows)
        } catch (reason) { setError(`终端布局失败：${errorMessage(reason)}`) }
      })
    }
    fit.current = doFit
    const observer = new ResizeObserver(doFit)
    observer.observe(host.current)
    const input = instance.onData(data => controller.write(data))
    void document.fonts.ready.then(() => { if (!disposed) doFit() })
    doFit()
    if (tab.sessionId) void controller.resume(tab.agent, tab.sessionId)
    else void controller.start(tab.agent)
    return () => {
      disposed = true
      cancelAnimationFrame(frame)
      observer.disconnect()
      input.dispose()
      controller.dispose()
      instance.dispose()
      term.current = null
    }
    // restart only when the tab identity changes; font/theme/activity flow through refs + effects
  }, [bridge, tab.key, tab.agent, tab.sessionId])

  useEffect(() => {
    if (term.current) term.current.options.fontSize = fontSize
    fit.current()
  }, [fontSize])

  useEffect(() => {
    if (term.current) term.current.options.theme = theme
  }, [theme])

  useEffect(() => {
    if (active) { fit.current(); term.current?.focus() }
  }, [active])

  return <div className={cn('relative min-h-0 flex-1', !active && 'hidden')}>
    <div ref={host} aria-label={`终端 ${tab.title}`} className="h-full w-full px-1 py-2" />
    {active && error && <p role="alert"
      className="absolute inset-x-3 bottom-2 rounded-md border border-destructive/40 bg-term/90 px-3 py-2 font-mono text-xs leading-5 text-destructive">
      {error}
    </p>}
  </div>
}

function EmptyState({ bridge }: { bridge: Bridge | null }) {
  const { t } = useI18n()
  return <div className="grid flex-1 place-items-center p-6">
    <div className="max-w-md text-center">
      <div className="flex justify-center gap-2" aria-hidden="true">
        {['⇧T', '⇧K', '⇧W', '⇥'].map(label =>
          <kbd key={label} className="rounded border border-border border-b-2 bg-card px-2 py-1 font-mono text-[11px] text-muted-foreground">Ctrl {label}</kbd>)}
      </div>
      <p className="mt-4 text-[15px] font-medium">{bridge ? t('terminal.empty.title') : t('terminal.empty.runtimeUnavailable')}</p>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">{bridge ? t('terminal.empty.desc') : t('terminal.empty.descBrowser')}</p>
    </div>
  </div>
}

export function TerminalPanel({ bridge, tabs, activeKey, theme, onPhase }: {
  bridge: Bridge | null
  tabs: Tab[]
  activeKey: number | null
  theme: 'dark' | 'light'
  onPhase: (key: number, phase: Phase) => void
}) {
  const { t } = useI18n()
  const [fontSize, setFontSize] = useState(14)
  const [paletteChoice, setPaletteChoice] = useState(storedPalette)
  const palette = resolvePalette(paletteChoice, theme)
  useEffect(() => {
    try { localStorage.setItem(terminalPaletteKey, paletteChoice) } catch { /* private mode: not persisted */ }
  }, [paletteChoice])
  const [dimensions, setDimensions] = useState('— × —')
  const active = tabs.find(tab => tab.key === activeKey) ?? null
  const running = active?.phase === 'running'

  return <div data-terminal-palette={palette.id} style={{
    backgroundColor: palette.theme.background,
    color: palette.theme.foreground,
    '--term': palette.theme.background,
    '--term-fg': palette.theme.foreground,
  } as React.CSSProperties}
    className="relative flex min-w-0 flex-1 flex-col overflow-hidden">
    <PalettePicker value={paletteChoice} onChange={setPaletteChoice} />
    {tabs.length === 0 || !bridge ? <EmptyState bridge={bridge} /> :
      tabs.map(tab => <TerminalView key={tab.key} bridge={bridge} tab={tab} active={tab.key === activeKey}
        fontSize={fontSize} theme={palette.theme} onPhase={onPhase} onDimensions={setDimensions} />)}
    <div className="flex h-8 shrink-0 items-center gap-4 border-t border-border bg-card px-3.5 font-mono text-[11px] text-muted-foreground">
      <span aria-hidden="true" className={cn('size-1.5 rounded-full', running ? 'bg-success' : 'bg-faint')} />
      <span className="truncate">
        {active ? `${agentNames[active.agent]} · ${active.title} · ${t(`terminal.phase.${active.phase}`)}` : t('terminal.status.none')}
      </span>
      <span className="ml-auto">{active ? dimensions : '— × —'}</span>
      <Button variant="ghost" className="size-6 px-0" aria-label={t('terminal.fontSize.decrease')}
        disabled={!bridge || fontSize <= 10} onClick={() => setFontSize(size => size - 1)}>
        <Minus aria-hidden="true" />
      </Button>
      <span className="min-w-6 text-center text-faint">{fontSize}</span>
      <Button variant="ghost" className="size-6 px-0" aria-label={t('terminal.fontSize.increase')}
        disabled={!bridge || fontSize >= 24} onClick={() => setFontSize(size => size + 1)}>
        <Plus aria-hidden="true" />
      </Button>
    </div>
  </div>
}
