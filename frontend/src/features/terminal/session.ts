import { translate, resolveLanguage, storedChoice } from '../../lib/i18n/core.ts'
import { decodeOutput, errorMessage } from '../../lib/bridge.ts'
import type { Agent, Bridge, Exit, Output } from '../../lib/bridge.ts'

function t(key: string, params?: Record<string, string | number>) {
  return translate(resolveLanguage(storedChoice()), key, params)
}

export type Phase = 'idle' | 'starting' | 'running' | 'stopping' | 'exited' | 'failed'
type Sink = { write(data: Uint8Array): void; reset(): void }
type Event = { kind: 'output'; payload: Output } | { kind: 'exit'; payload: Exit }

export class TerminalSession {
  private id: string | null = null
  private phase: Phase = 'idle'
  private disposed = false
  private unsubscribe: (() => void)[] = []
  private pending: Event[] = []
  private pendingSize = 0
  private overflow = false
  private input = Promise.resolve()
  private size = { cols: 80, rows: 24 }

  private bridge: Bridge
  private sink: Sink
  private update: (phase: Phase) => void
  private report: (message: string) => void

  constructor(bridge: Bridge, sink: Sink, update: (phase: Phase) => void, report: (message: string) => void) {
    this.bridge = bridge
    this.sink = sink
    this.update = update
    this.report = report
  }

  private setPhase(phase: Phase) {
    this.phase = phase
    if (!this.disposed) this.update(phase)
  }

  private detach() {
    this.unsubscribe.splice(0).forEach(off => off())
    this.pending = []
    this.pendingSize = 0
  }

  private receive(event: Event) {
    if (this.disposed) return
    if (!this.id) {
      // shortcut: buffer up to 4 MiB before the ID resolves; use backend flow control for production.
      this.pendingSize += event.kind === 'output' ? event.payload.data.length : 1
      if (this.pendingSize > 4 * 1024 * 1024) {
        this.overflow = true
        this.pending = []
      } else if (!this.overflow) this.pending.push(event)
      return
    }
    if (event.payload.id !== this.id) return
    if (event.kind === 'output') {
      try { this.sink.write(decodeOutput(event.payload.data)) }
      catch (error) { this.report(t('terminal.error.decodeFailed', { error: errorMessage(error) })) }
    } else {
      this.id = null
      this.setPhase('exited')
      if (event.payload.error) this.report(t('terminal.error.exited', { error: event.payload.error }))
      this.detach()
    }
  }

  async start(agent: Agent) {
    if (this.disposed || ['starting', 'stopping'].includes(this.phase)) return
    if (this.phase === 'running') return
    await this.begin(() => this.bridge.app.StartTerminal(agent))
  }

  // resume starts this tab's own terminal; other tabs' sessions are independent.
  async resume(agent: Agent, sessionId: string) {
    if (this.disposed || ['starting', 'stopping'].includes(this.phase)) return
    await this.begin(() => this.bridge.app.ResumeSession(agent, sessionId))
  }

  private async begin(call: () => Promise<string>) {
    this.detach()
    this.overflow = false
    this.sink.reset()
    this.setPhase('starting')
    try {
      this.unsubscribe.push(this.bridge.runtime.EventsOn('terminal:output', payload =>
        this.receive({ kind: 'output', payload })))
      this.unsubscribe.push(this.bridge.runtime.EventsOn('terminal:exit', payload =>
        this.receive({ kind: 'exit', payload })))
      const id = await call()
      if (!id) throw new Error('后端未返回终端 ID')
      if (this.disposed) {
        await this.bridge.app.CloseTerminal(id)
        return
      }
      this.id = id
      this.setPhase('running')
      if (this.overflow) {
        this.report(t('terminal.error.bufferOverflow'))
        await this.close()
        return
      }
      const pending = this.pending.splice(0)
      this.pendingSize = 0
      for (const event of pending) this.receive(event)
      if (this.id) await this.resize(this.size.cols, this.size.rows)
    } catch (error) {
      this.report(t('terminal.error.startFailed', { error: errorMessage(error) }))
      if (!this.id) { this.detach(); this.setPhase('failed') }
    }
  }

  write(data: string) {
    const id = this.id
    if (!id || this.phase !== 'running') return
    this.input = this.input.then(async () => {
      if (this.id === id && this.phase === 'running' && !this.disposed)
        await this.bridge.app.WriteTerminal(id, data)
    }).catch(error => this.report(t('terminal.error.inputFailed', { error: errorMessage(error) })))
  }

  async resize(cols: number, rows: number) {
    if (!Number.isInteger(cols) || !Number.isInteger(rows) || cols < 1 || rows < 1) return
    this.size = { cols, rows }
    const id = this.id
    if (!id || this.phase !== 'running') return
    try { await this.bridge.app.ResizeTerminal(id, cols, rows) }
    catch (error) {
      if (this.id === id && !this.disposed) this.report(t('terminal.error.resizeFailed', { error: errorMessage(error) }))
    }
  }

  async close() {
    const id = this.id
    if (!id || this.phase !== 'running') return
    this.setPhase('stopping')
    try {
      await this.bridge.app.CloseTerminal(id)
      if (this.id === id) {
        this.id = null
        this.detach()
        this.setPhase('exited')
      }
    } catch (error) {
      if (this.id === id) this.setPhase('running')
      this.report(t('terminal.error.closeFailed', { error: errorMessage(error) }))
    }
  }

  dispose() {
    this.disposed = true
    this.detach()
    const id = this.id
    this.id = null
    if (id) void this.bridge.app.CloseTerminal(id).catch(error =>
      this.report(t('terminal.error.cleanupFailed', { error: errorMessage(error) })))
  }
}
