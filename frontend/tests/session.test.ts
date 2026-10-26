import { test } from 'node:test'
import assert from 'node:assert/strict'
import { TerminalSession } from '../src/features/terminal/session.ts'
import { decodeOutput } from '../src/lib/bridge.ts'
import type { Bridge, Exit, Output, Runtime } from '../src/lib/bridge.ts'

function setup() {
  let resolveStart!: (id: string) => void
  // the real Wails runtime fans an event out to every subscriber; the mock must too
  const listeners = new Map<string, Set<(payload: never) => void>>()
  const writes: Uint8Array[] = []
  const phases: string[] = []
  const errors: string[] = []
  const closed: string[] = []
  const inputs: string[] = []
  const sizes: number[][] = []
  let starts = 0
  const bridge: Bridge = {
    runtime: { EventsOn(name: string, callback: (payload: never) => void) {
      const set = listeners.get(name) ?? new Set<(payload: never) => void>()
      set.add(callback)
      listeners.set(name, set)
      return () => {
        set.delete(callback)
        if (!set.size) listeners.delete(name)
      }
    } } as Runtime,
    app: {
      StartTerminal: () => {
        starts++
        assert.equal(listeners.size, 2, 'listeners must exist before start')
        return new Promise<string>(resolve => { resolveStart = resolve })
      },
      ResumeSession: () => {
        starts++
        return new Promise<string>(resolve => { resolveStart = resolve })
      },
      ListSessions: async () => [],
      WriteTerminal: async (_id, data) => { inputs.push(data) },
      ResizeTerminal: async (_id, cols, rows) => { sizes.push([cols, rows]) },
      CloseTerminal: async id => { closed.push(id) },
      DetectEnvironment: async () => [],
      ExitApp: async () => {},
      OpenInWindowsTerminal: async () => {},
      ConfigurationPaths: async () => ({ WT: '', PowerShell: '', Operations: '' }),
      PlanConfiguration: async () => ({ PlanID: 'test', Diffs: [], Blocked: false }),
      ExecuteConfiguration: async () => ({ OpID: 'test', Steps: [], Succeeded: true }),
      ListConfigurationOperations: async () => [],
      DetectInventory: async () => ({ Items: [], Sources: [], Note: '' }),
  InventoryPaths: async () => ({ Ledger: '' }),
  PlanInventoryRemoval: async () => ({ Token: 'preview-rm-test', Plan: { Steps: [], Blocked: false, Conflicts: [] } }),
  ExecuteInventoryRemoval: async () => ({ Token: 'preview-rm-test', OperationID: 'op-rm-test', State: 'succeeded', Records: [] }),
  PlanInstall: async () => ({ Token: 'preview-test', Plan: { Steps: [], Blocked: false, Conflicts: [] } }),
  ExecuteInstall: async () => ({ Token: 'op-test', OperationID: 'op-test', State: 'succeeded', Records: [] }),
  ReconcileInventory: async () => [],
  PlanConfigurationRestore: async () => ({ PlanID: 'test', Diffs: [], Blocked: false }),
    },
  }
  const session = new TerminalSession(bridge, {
    write: data => { writes.push(data) }, reset: () => { writes.length = 0 },
  }, phase => phases.push(phase), error => errors.push(error))
  return { session, bridge, writes, phases, errors, closed, inputs, sizes, listeners,
    starts: () => starts, resolve: (id = 'one') => resolveStart(id),
    output: (payload: Output) => listeners.get('terminal:output')?.forEach(send => send(payload as never)),
    exit: (payload: Exit) => listeners.get('terminal:exit')?.forEach(send => send(payload as never)) }
}

const encoded = (bytes: number[]) => btoa(String.fromCharCode(...bytes))

test('preserves split UTF-8 bytes and filters early events by ID', async () => {
  const t = setup()
  const start = t.session.start('powershell')
  await t.session.start('codex')
  assert.equal(t.starts(), 1)
  t.output({ id: 'other', data: encoded([65]) })
  t.output({ id: 'one', data: encoded([0xe4, 0xb8]) })
  t.output({ id: 'one', data: encoded([0xad]) })
  t.resolve()
  await start
  assert.equal(new TextDecoder().decode(Uint8Array.from(t.writes.flatMap(data => [...data]))), '中')
  assert.deepEqual(t.sizes, [[80, 24]])
  await t.session.resize(0, 24)
  assert.equal(t.sizes.length, 1)
  t.session.write('a')
  t.session.write('中')
  await new Promise(resolve => setImmediate(resolve))
  assert.deepEqual(t.inputs, ['a', '中'])
  await t.session.close()
  assert.deepEqual(t.closed, ['one'])
  assert.equal(t.listeners.size, 0)
})

test('handles exit before the start promise resolves', async () => {
  const t = setup()
  const start = t.session.start('claude')
  t.output({ id: 'one', data: encoded([65]) })
  t.exit({ id: 'one', error: 'process exited' })
  t.resolve()
  await start
  assert.deepEqual(t.phases, ['starting', 'running', 'exited'])
  assert.equal(t.writes.length, 1)
  assert.equal(t.sizes.length, 0)
  assert.match(t.errors[0], /process exited/)
  assert.equal(t.listeners.size, 0)
})

test('unmount during start closes the returned terminal and unsubscribes', async () => {
  const t = setup()
  const start = t.session.start('codex')
  t.session.dispose()
  t.resolve()
  await start
  assert.deepEqual(t.closed, ['one'])
  assert.equal(t.listeners.size, 0)
})

test('reports malformed base64 and supports retrying a failed close', async () => {
  assert.throws(() => decodeOutput('%%%'))
  const t = setup()
  const start = t.session.start('powershell')
  t.resolve()
  await start
  t.output({ id: 'one', data: '%%%' })
  t.bridge.app.CloseTerminal = async () => { throw new Error('busy') }
  await t.session.close()
  assert.equal(t.phases.at(-1), 'running')
  assert.equal(t.errors.length, 2)
  t.bridge.app.CloseTerminal = async id => { t.closed.push(id) }
  await t.session.close()
  assert.equal(t.phases.at(-1), 'exited')
})

test('startup buffer overflow closes the process without replaying partial output', async () => {
  const t = setup()
  const start = t.session.start('powershell')
  t.output({ id: 'one', data: 'A'.repeat(4 * 1024 * 1024 + 1) })
  t.resolve()
  await start
  assert.deepEqual(t.closed, ['one'])
  assert.equal(t.writes.length, 0)
  assert.equal(t.phases.at(-1), 'exited')
  assert.match(t.errors[0], /缓冲上限/)
})

test('failed start removes listeners and can be retried', async () => {
  const t = setup()
  const original = t.bridge.app.StartTerminal
  t.bridge.app.StartTerminal = async () => { throw new Error('not installed') }
  await t.session.start('claude')
  assert.equal(t.phases.at(-1), 'failed')
  assert.equal(t.listeners.size, 0)
  t.bridge.app.StartTerminal = original
  const retry = t.session.start('powershell')
  t.resolve()
  await retry
  assert.equal(t.phases.at(-1), 'running')
  t.session.dispose()
})

test('two sessions multiplex the event stream by terminal id', async () => {
  const t = setup()
  const first = t.session.start('claude')
  t.resolve()
  await first
  const writesB: Uint8Array[] = []
  const second = new TerminalSession(t.bridge,
    { write: data => writesB.push(data), reset: () => { writesB.length = 0 } }, () => {}, () => {})
  const resume = second.resume('codex', 'session-42')
  await new Promise(resolve => setImmediate(resolve))
  assert.deepEqual(t.closed, [], 'resume must not close other running terminals')
  t.resolve('two')
  await resume
  t.output({ id: 'one', data: encoded([65]) })
  t.output({ id: 'two', data: encoded([66]) })
  assert.equal(t.writes.length, 1, 'output for the other tab must not land in this session')
  assert.equal(writesB.length, 1)
  second.dispose()
  t.session.dispose()
})
