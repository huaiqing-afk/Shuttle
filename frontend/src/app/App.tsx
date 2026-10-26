import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ConfigurationPanel } from '../features/configuration/ConfigurationPanel'
import { InventoryPanel } from '../features/inventory/InventoryPanel'
import { ArrowRight, PanelLeft } from 'lucide-react'
import { Button } from '../components/ui/button'
import { EnvFooter } from './EnvFooter'
import { ThemeSeg } from './ThemeSeg'
import { LangSeg } from './LangSeg'
import { useI18n } from '../lib/i18n'
import { useTheme } from './theme'
import { TerminalTabs } from '../features/terminal/TerminalTabs'
import { TerminalPanel } from '../features/terminal/TerminalPanel'
import type { Phase } from '../features/terminal/session'
import { agentNames, type Tab } from '../features/terminal/types'
import { SessionsPanel } from '../features/sessions/SessionsPanel'
import { getBridge, type Agent, type Session } from '../lib/bridge'

const maxTabs = 8
// sidebar visibility survives restarts, like the theme choice
const SIDEBAR_KEY = 'shuttle-sidebar'
function storedSidebarCollapsed(): boolean {
  try { return localStorage.getItem(SIDEBAR_KEY) === 'collapsed' } catch { return false }
}

export function App() {
  const { t } = useI18n()
  const [bridge] = useState(getBridge)
  const { mode, resolved, setMode } = useTheme()
  const [tabs, setTabs] = useState<Tab[]>([])
  const [activeKey, setActiveKey] = useState<number | null>(null)
  const [searchSignal, setSearchSignal] = useState(0)
  const [confirmExit, setConfirmExit] = useState<number | null>(null)
  const [configurationOpen, setConfigurationOpen] = useState(false)
  const [inventoryOpen, setInventoryOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(storedSidebarCollapsed)
  const keySeq = useRef(0)

  useEffect(() => {
    try { localStorage.setItem(SIDEBAR_KEY, sidebarCollapsed ? 'collapsed' : 'open') } catch { /* private mode: not persisted */ }
  }, [sidebarCollapsed])

  const setPhase = useCallback((key: number, phase: Phase) => {
    setTabs(list => list.map(tab => tab.key === key ? { ...tab, phase } : tab))
  }, [])

  function openTerminal(agent: Agent, options?: { title?: string; sessionId?: string }) {
    if (tabs.length >= maxTabs) return
    const key = ++keySeq.current
    setTabs([...tabs, {
      key, agent, title: options?.title ?? agentNames[agent], sessionId: options?.sessionId, phase: 'starting',
    }])
    setActiveKey(key)
  }

  function openSession(session: Session) {
    // a session already open in a tab is focused, never opened twice
    const open = tabs.find(tab => tab.sessionId === session.id)
    if (open) { setActiveKey(open.key); return }
    const agent = session.agent === 'codex' || session.agent === 'claude' ? session.agent : 'powershell'
    openTerminal(agent, { title: session.title || session.id.slice(0, 8), sessionId: session.id })
  }

  function confirmClose(tab: Tab): boolean {
    return (tab.phase !== 'running' && tab.phase !== 'starting') ||
      window.confirm(t('app.tab.closeConfirm', { title: tab.title }))
  }

  // batch close shared by single close / close others / close right / close all:
  // one cancelled confirm aborts the whole batch, focus falls to the nearest
  // surviving tab at or after the first closed one
  function closeMany(keys: number[]) {
    const doomed = new Set(keys)
    if (!doomed.size) return
    const anchor = tabs.findIndex(tab => doomed.has(tab.key))
    for (const tab of tabs) if (doomed.has(tab.key) && !confirmClose(tab)) return
    const rest = tabs.filter(tab => !doomed.has(tab.key))
    let fallback: number | null = null
    for (let i = anchor + 1; i < tabs.length && fallback === null; i++) {
      if (!doomed.has(tabs[i].key)) fallback = tabs[i].key
    }
    for (let i = anchor - 1; i >= 0 && fallback === null; i--) {
      if (!doomed.has(tabs[i].key)) fallback = tabs[i].key
    }
    setTabs(rest)
    if (activeKey === null || doomed.has(activeKey)) setActiveKey(fallback)
  }

  const openSessionKeys = useMemo(() => {
    const map = new Map<string, number>()
    for (const tab of tabs) if (tab.sessionId) map.set(tab.sessionId, tab.key)
    return map
  }, [tabs])

  // the window ✕ with terminals running raises this from Go's beforeClose
  useEffect(() => {
    if (!bridge) return
    return bridge.runtime.EventsOn('app:close-requested', runningTerminals => setConfirmExit(runningTerminals))
  }, [bridge])

  // rebound each render so handlers always see current tabs/activeKey.
  // Keys use Ctrl+Shift/Alt variants: a focused terminal must receive plain Ctrl+letters itself.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (configurationOpen || inventoryOpen) return
      if (event.key === 'Escape' && confirmExit !== null) { setConfirmExit(null); return }
      if (event.ctrlKey && !event.altKey && !event.metaKey && event.key === 'Tab' && tabs.length > 1) {
        event.preventDefault()
        const index = Math.max(0, tabs.findIndex(tab => tab.key === activeKey))
        const next = event.shiftKey ? index + tabs.length - 1 : index + 1
        setActiveKey(tabs[next % tabs.length].key)
        return
      }
      // direct tab jump keys (Alt+digit) collide with common screenshot/launcher tools
      if (!event.ctrlKey || !event.shiftKey || event.altKey || event.metaKey) return
      const key = event.key.toLowerCase()
      if (key === 't') { event.preventDefault(); openTerminal('powershell') }
      else if (key === 'w') { if (activeKey !== null) { event.preventDefault(); closeMany([activeKey]) } }
      else if (key === 'k') { event.preventDefault(); setSearchSignal(signal => signal + 1) }
      else if (key === 'b') { event.preventDefault(); setSidebarCollapsed(collapsed => !collapsed) }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  })

  return <div className="flex h-screen flex-col overflow-hidden">
    <header className="relative flex h-10 shrink-0 items-center gap-3 border-b border-border bg-card px-4">
      <Button variant="ghost" className="h-7 w-7 px-0" aria-label={sidebarCollapsed ? t('app.expandSidebar') : t('app.collapseSidebar')}
        aria-expanded={!sidebarCollapsed} onClick={() => setSidebarCollapsed(collapsed => !collapsed)}>
        <PanelLeft aria-hidden="true" />
      </Button>
      <span aria-hidden="true" className="grid size-[22px] place-items-center rounded-md bg-primary text-primary-foreground">
        <ArrowRight className="size-3" strokeWidth={2.5} />
      </span>
      <span className="font-mono text-[13px]">shuttle<span className="ml-1 text-faint">{t('app.tagline')}</span></span>
      <span className="rounded-full border border-border px-2 py-0.5 font-mono text-[11px] text-faint">
        {bridge ? t('app.versionBadge') : t('app.browserPreview')}
      </span>
      <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
        <ThemeSeg mode={mode} onChange={setMode} />
      </div>
      <div className="ml-auto flex items-center gap-3">
        <LangSeg />
        <Button variant="outline" className="h-7 px-3 text-[13px]" onClick={() => setInventoryOpen(true)}>{t('app.btn.inventory')}</Button>
        <Button variant="outline" className="h-7 px-3 text-[13px]" onClick={() => setConfigurationOpen(true)}>{t('app.btn.config')}</Button>
      </div>
    </header>
    <div className="flex min-h-0 flex-1">
      {!sidebarCollapsed && <aside className="flex w-[272px] min-w-[272px] flex-col border-r border-border bg-card">
        <SessionsPanel bridge={bridge} openSessionKeys={openSessionKeys} activeKey={activeKey}
          onPick={openSession} focusSignal={searchSignal} />
        <EnvFooter bridge={bridge} />
      </aside>}
      <main className="flex min-w-0 flex-1 flex-col">
        <TerminalTabs tabs={tabs} activeKey={activeKey} limit={maxTabs}
          onSelect={setActiveKey} onCloseMany={closeMany} onNew={agent => openTerminal(agent)} />
        <TerminalPanel bridge={bridge} tabs={tabs} activeKey={activeKey} theme={resolved} onPhase={setPhase} />
      </main>
      </div>
      {configurationOpen && <ConfigurationPanel bridge={bridge} onClose={() => setConfigurationOpen(false)} />}
      {inventoryOpen && <InventoryPanel bridge={bridge} onClose={() => setInventoryOpen(false)} />}
      {confirmExit !== null && <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-[2px]"
        onClick={() => setConfirmExit(null)}>
        <div role="alertdialog" aria-modal="true" aria-labelledby="exit-title"
          onClick={event => event.stopPropagation()}
          className="w-[min(400px,92vw)] rounded-xl border border-border-strong bg-hover p-5 shadow-2xl">
          <h3 id="exit-title" className="text-[15px] font-semibold">{t('app.dialog.exit.title')}</h3>
          <p className="mt-2.5 text-sm leading-6 text-muted-foreground">
            {t('app.dialog.exit.desc', { count: confirmExit })}
          </p>
          <div className="mt-5 flex justify-end gap-2.5">
            <Button variant="outline" onClick={() => setConfirmExit(null)}>{t('app.dialog.exit.cancel')}</Button>
            <Button onClick={() => { setConfirmExit(null); void bridge?.app.ExitApp() }}>{t('app.dialog.exit.confirm')}</Button>
          </div>
        </div>
      </div>}
  </div>
}
