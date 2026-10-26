import { useCallback, useEffect, useRef, useState } from 'react'
import { Check, ChevronRight, ExternalLink, Search } from 'lucide-react'
import { cn } from '../../lib/utils'
import { errorMessage, type Bridge, type Session } from '../../lib/bridge'
import { useI18n } from '../../lib/i18n'

type AgentFilter = 'all' | 'claude' | 'codex'
const agentMeta: Record<string, { short: string; chip: string; text: string; chipBg: string; dot: string }> = {
  claude: { short: 'Claude', chip: 'CL', text: 'text-claude', chipBg: 'bg-claude/15 text-claude', dot: 'bg-claude' },
  codex: { short: 'Codex', chip: 'CX', text: 'text-codex', chipBg: 'bg-codex/15 text-codex', dot: 'bg-codex' },
  powershell: { short: 'PowerShell', chip: 'PS', text: 'text-shell', chipBg: 'bg-shell/15 text-shell', dot: 'bg-shell' },
}
function metaOf(agent: string) {
  return agentMeta[agent] ?? { short: agent, chip: agent.slice(0, 2).toUpperCase(), text: 'text-shell', chipBg: 'bg-shell/15 text-shell', dot: 'bg-shell' }
}

function relativeTime(ms: number, t: (k: string, p?: Record<string, string | number>) => string): string {
  const minutes = Math.max(0, Math.round((Date.now() - ms) / 60000))
  if (minutes < 1) return t('sessions.time.justNow')
  if (minutes < 60) return t('sessions.time.minutesAgo', { count: minutes })
  const hours = Math.round(minutes / 60)
  if (hours < 24) return t('sessions.time.hoursAgo', { count: hours })
  return t('sessions.time.daysAgo', { count: Math.round(hours / 24) })
}

export function SessionsPanel({ bridge, openSessionKeys, activeKey, onPick, focusSignal }: {
  bridge: Bridge | null
  openSessionKeys: Map<string, number>
  activeKey: number | null
  onPick: (session: Session) => void
  focusSignal: number
}) {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<AgentFilter>('all')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const input = useRef<HTMLInputElement>(null)
  const seq = useRef(0)
  const loadingRef = useRef(false)
  const lastLoad = useRef(0)

  const load = useCallback(async () => {
    if (!bridge || loadingRef.current) return
    loadingRef.current = true
    lastLoad.current = Date.now()
    setError('')
    try {
      const result = await bridge.app.ListSessions()
      if (!Array.isArray(result)) throw new Error(t('sessions.formatError'))
      setSessions(result)
    } catch (reason) { setError(t('sessions.loadFailed', { error: errorMessage(reason) })) }
    finally { loadingRef.current = false; setLoading(false) }
  }, [bridge])

  useEffect(() => { void load() }, [load])
  // sessions are read-only disk data: refresh on window focus instead of a manual button
  useEffect(() => {
    if (!bridge) return
    const onFocus = () => { if (Date.now() - lastLoad.current > 30000) void load() }
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [bridge, load])
  useEffect(() => { if (focusSignal > 0) input.current?.focus() }, [focusSignal])

  const text = query.trim().toLowerCase()
  const counts: Record<AgentFilter, number> = { all: sessions?.length ?? 0, claude: 0, codex: 0 }
  const groups = new Map<string, Session[]>()
  for (const session of sessions ?? []) {
    if (session.agent === 'claude' || session.agent === 'codex') counts[session.agent]++
    if (filter !== 'all' && session.agent !== filter) continue
    if (text && !(session.title.toLowerCase().includes(text) || session.workspace.toLowerCase().includes(text) ||
      session.name.toLowerCase().includes(text) || session.gitBranch.includes(text))) continue
    const list = groups.get(session.workspace) ?? []
    list.push(session)
    groups.set(session.workspace, list)
  }

  return <div className="flex min-h-0 flex-1 flex-col">
    <div className="shrink-0 px-3.5 pb-2.5 pt-3.5">
      <div className="relative flex items-center">
        <Search className="pointer-events-none absolute left-2.5 size-3.5 text-faint" aria-hidden="true" />
        <input ref={input} type="text" value={query} onChange={event => setQuery(event.target.value)}
          placeholder={t('sessions.searchPlaceholder')} aria-label={t('sessions.searchLabel')}
          className="h-[34px] w-full rounded-lg border border-border bg-background pl-8 pr-14 font-mono text-xs text-foreground placeholder:text-faint focus:border-border-strong focus:outline-none focus:ring-2 focus:ring-primary/20" />
        <kbd className="pointer-events-none absolute right-2 rounded border border-border px-1.5 py-0.5 font-mono text-[10px] text-faint">Ctrl K</kbd>
      </div>
      <div className="mt-2.5 flex items-center gap-1.5">
        {(['all', 'claude', 'codex'] as AgentFilter[]).map(id => {
          const meta = id === 'all' ? null : metaOf(id)
          return <button key={id} aria-pressed={filter === id} disabled={sessions === null}
            onClick={() => setFilter(id)}
            className={cn('flex h-[26px] min-w-0 flex-1 items-center justify-center gap-1.5 rounded-md border px-1 font-mono text-[11px]',
              filter === id ? 'border-border-strong bg-elevated text-foreground'
                : 'border-border text-muted-foreground hover:border-border-strong hover:text-foreground',
              sessions === null && 'opacity-40')}>
            {meta && <span aria-hidden="true" className={cn('size-[6px] shrink-0 rounded-full', meta.dot)} />}
            <span className="truncate">{meta ? meta.short : t('sessions.all')}</span>
            <span className="shrink-0 text-faint">{counts[id]}</span>
          </button>
        })}
      </div>
    </div>
    <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
      {!bridge ? <p className="p-3 text-xs leading-6 text-faint">{t('sessions.runtimeUnavailable')}</p>
        : sessions === null ? <p role="status" className="p-3 text-xs text-faint">{t('sessions.loading')}</p>
        : groups.size === 0 ? <p className="p-3 text-xs leading-6 text-faint">
          {query || filter !== 'all' ? t('sessions.noMatches') : t('sessions.noneFound')}</p>
        : [...groups.entries()].map(([workspace, list]) => {
          const isCollapsed = collapsed.has(workspace)
          return <section key={workspace} className="mt-1.5">
            <div className="flex w-full items-center gap-1 rounded-md px-1.5 py-1 text-left font-mono text-[11px] text-faint hover:text-muted-foreground">
              <button aria-expanded={!isCollapsed}
                onClick={() => setCollapsed(prev => {
                  const next = new Set(prev)
                  if (next.has(workspace)) next.delete(workspace)
                  else next.add(workspace)
                  return next
                })}
                className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
                <ChevronRight aria-hidden="true" className={cn('size-3 shrink-0 transition-transform', !isCollapsed && 'rotate-90')} />
                <span className="truncate" title={workspace}>{workspace}</span>
                <span className="rounded-full border border-border px-1.5 text-[10px]">{list.length}</span>
              </button>
              <button type="button"
                aria-label={t('sessions.openInWT', { workspace })}
                title={t('sessions.openInWTTitle')}
                onClick={async () => {
                  try {
                    await bridge?.app.OpenInWindowsTerminal(workspace)
                  } catch (err) {
                    setError(errorMessage(err))
                  }
                }}
                className="grid size-5 shrink-0 place-items-center rounded text-faint hover:bg-elevated hover:text-foreground">
                <ExternalLink className="size-3" aria-hidden="true" />
              </button>
            </div>
            {!isCollapsed && <ul className="flex flex-col gap-0.5">
              {list.map(session => {
                const meta = metaOf(session.agent)
                const openKey = openSessionKeys.get(session.id)
                const isOpen = openKey !== undefined
                const title = session.name || session.title || session.id.slice(0, 8)
                return <li key={`${session.agent}-${session.id}`}>
                  <button disabled={loading} onClick={() => onPick({ ...session, title })}
                    aria-current={openKey === activeKey}
                    className={cn('flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left',
                      openKey === activeKey ? 'bg-elevated shadow-[inset_2px_0_0_var(--primary)]'
                        : 'hover:bg-hover')}>
                    <span aria-hidden="true"
                      className={cn('grid size-5 shrink-0 place-items-center rounded-md font-mono text-[10px] font-bold', meta.chipBg)}>
                      {meta.chip}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] text-foreground" title={title}>{title}</span>
                      <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-[10.5px] font-mono leading-4 text-faint">
                        <span className={meta.text}>{meta.short}</span>
                        <span>{relativeTime(session.lastActive, t)}</span>
                        {session.running && session.status &&
                          <span className={session.status === 'busy' ? 'text-primary' : 'text-success'}>{session.status}</span>}
                        {session.gitBranch && session.gitBranch !== 'HEAD' && <span className="truncate">{session.gitBranch}</span>}
                      </span>
                    </span>
                    {isOpen && <Check aria-label={t('sessions.opened')} className="size-3.5 shrink-0 text-primary" />}
                  </button>
                </li>
              })}
            </ul>}
          </section>
        })}
      {error && <p role="alert" className="p-3 text-xs leading-5 text-destructive">{error}</p>}
    </div>
    <div className="shrink-0 border-t border-border px-3.5 py-2.5 text-[11px] leading-5 text-faint">
      {t('sessions.readOnlyNotice')}
    </div>
  </div>
}
