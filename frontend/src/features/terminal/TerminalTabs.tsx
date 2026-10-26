import { useEffect, useRef, useState } from 'react'
import { Plus, X } from 'lucide-react'
import { cn } from '../../lib/utils'
import type { Agent } from '../../lib/bridge'
import { agentNames, type Tab } from './types'
import { useI18n } from '../../lib/i18n'

export const newTerminalChoices: { id: Agent; name: string }[] = [
  { id: 'powershell', name: 'PowerShell' },
  { id: 'claude', name: 'Claude Code' },
  { id: 'codex', name: 'Codex' },
]

const runningDot: Record<string, string> = {
  claude: 'bg-claude shadow-[0_0_6px_var(--claude)]',
  codex: 'bg-codex shadow-[0_0_6px_var(--codex)]',
  powershell: 'bg-success shadow-[0_0_6px_var(--success)]',
}

export function TerminalTabs({ tabs, activeKey, limit, onSelect, onCloseMany, onNew }: {
  tabs: Tab[]
  activeKey: number | null
  limit: number
  onSelect: (key: number) => void
  onCloseMany: (keys: number[]) => void
  onNew: (agent: Agent) => void
}) {
  const { t } = useI18n()
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const [ctx, setCtx] = useState<{ x: number; y: number; key: number } | null>(null)

  useEffect(() => {
    if (!menuOpen) return
    const dismiss = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('click', dismiss)
    return () => document.removeEventListener('click', dismiss)
  }, [menuOpen])

  // right-click menu: any click, another right-click (React already
  // preventDefault'd the opening one) or Escape dismisses it
  useEffect(() => {
    if (!ctx) return
    const dismiss = (event: MouseEvent) => { if (!event.defaultPrevented) setCtx(null) }
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setCtx(null) }
    document.addEventListener('click', dismiss)
    document.addEventListener('contextmenu', dismiss)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('click', dismiss)
      document.removeEventListener('contextmenu', dismiss)
      document.removeEventListener('keydown', onKey)
    }
  }, [ctx])

  const ctxIndex = ctx ? tabs.findIndex(tab => tab.key === ctx.key) : -1
  // skip entries that would close nothing
  const ctxItems = ctx && ctxIndex >= 0 ? [
    { label: t('terminal.close'), keys: [ctx.key] },
    ...(tabs.length > 1 ? [{ label: t('terminal.closeOthers'), keys: tabs.filter(tab => tab.key !== ctx.key).map(tab => tab.key) }] : []),
    ...(ctxIndex < tabs.length - 1 ? [{ label: t('terminal.closeRight'), keys: tabs.slice(ctxIndex + 1).map(tab => tab.key) }] : []),
    ...(tabs.length > 1 ? [{ label: t('terminal.closeAll'), keys: tabs.map(tab => tab.key) }] : []),
  ] : []

  return <div role="tablist" aria-label={t('terminal.tabsLabel')}
    className="flex h-9 shrink-0 items-stretch gap-0.5 border-b border-border bg-card px-2">
    {tabs.map(tab => {
      const active = tab.key === activeKey
      const running = tab.phase === 'running' || tab.phase === 'starting'
      // plain tabs are titled with the agent name already — don't repeat it
      const title = tab.title === agentNames[tab.agent] ? null : tab.title
      return <button key={tab.key} role="tab" aria-selected={active}
        onClick={() => onSelect(tab.key)}
        onAuxClick={event => { if (event.button === 1) onCloseMany([tab.key]) }}
        onContextMenu={event => {
          event.preventDefault()
          setCtx({ x: event.clientX, y: event.clientY, key: tab.key })
        }}
        className={cn('relative flex min-w-0 max-w-56 items-center gap-2 rounded-t-lg px-3 font-mono text-xs',
          active ? 'z-10 bg-term text-term-fg' : 'text-muted-foreground hover:bg-hover hover:text-foreground')}>
        <span aria-hidden="true"
          className={cn('size-[7px] shrink-0 rounded-full', running ? runningDot[tab.agent] : 'bg-faint')} />
        <span className="truncate">{agentNames[tab.agent]}{title && <span className="text-faint"> · {title}</span>}</span>
        <span className="shrink-0 text-[11px] text-faint">{t(`terminal.phase.${tab.phase}`)}</span>
        <span role="button" aria-label={`${t('terminal.close')} ${tab.title}`} tabIndex={0}
          onClick={event => { event.stopPropagation(); onCloseMany([tab.key]) }}
          onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.stopPropagation(); onCloseMany([tab.key]) } }}
          className="grid size-[18px] shrink-0 place-items-center rounded text-faint hover:bg-elevated hover:text-foreground">
          <X className="size-3" aria-hidden="true" />
        </span>
        {active && tab.phase !== 'starting' && <span aria-hidden="true"
          className="absolute inset-x-2 bottom-0 h-0.5 rounded bg-primary" />}
        {tab.phase === 'starting' && <span aria-hidden="true"
          className="absolute inset-x-0 bottom-0 h-0.5 overflow-hidden">
          <i className="tab-loading block h-full w-[36%] bg-gradient-to-r from-transparent via-primary to-transparent" />
        </span>}
      </button>
    })}
    <div ref={menuRef} className="relative ml-auto flex items-center">
      <button aria-label={t('terminal.new')} aria-haspopup="menu" aria-expanded={menuOpen}
        disabled={tabs.length >= limit} onClick={() => setMenuOpen(open => !open)}
        className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-hover hover:text-foreground disabled:opacity-40">
        <Plus className="size-4" aria-hidden="true" />
      </button>
      {menuOpen && <div role="menu"
        className="absolute right-0 top-9 z-40 min-w-44 rounded-lg border border-border-strong bg-hover p-1 shadow-[0_12px_32px_var(--shadow-1,rgba(0,0,0,0.5))]">
        {newTerminalChoices.map(item => <button key={item.id} role="menuitem"
          onClick={() => { setMenuOpen(false); onNew(item.id) }}
          className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm text-foreground hover:bg-elevated">
          {item.name}
        </button>)}
      </div>}
    </div>
    {ctx && ctxItems.length > 0 && <div role="menu"
      style={{ left: Math.min(ctx.x, window.innerWidth - 190), top: ctx.y }}
      className="fixed z-50 min-w-44 rounded-lg border border-border-strong bg-hover p-1 shadow-[0_12px_32px_var(--shadow-1,rgba(0,0,0,0.5))]">
      {ctxItems.map(item => <button key={item.label} role="menuitem"
        onClick={() => { setCtx(null); onCloseMany(item.keys) }}
        className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm text-foreground hover:bg-elevated">
        {item.label}
      </button>)}
    </div>}
  </div>
}
