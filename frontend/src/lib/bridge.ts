import type { ConfigurationBackend } from './configuration'
import type { InventoryBackend } from './inventory'

export type Agent = 'powershell' | 'claude' | 'codex'
export type Detection = { name: string; path: string; available: boolean }
export type Output = { id: string; data: string }
export type Exit = { id: string; error?: string }
export type Session = {
  agent: string; id: string; title: string; workspace: string; model: string
  turns: number; lastActive: number; gitBranch: string; running: boolean
  status: string; name: string
}
export interface Backend extends ConfigurationBackend, InventoryBackend {
  StartTerminal(agent: string): Promise<string>
  ResumeSession(agent: string, id: string): Promise<string>
  ListSessions(): Promise<Session[]>
  WriteTerminal(id: string, data: string): Promise<void>
  ResizeTerminal(id: string, cols: number, rows: number): Promise<void>
  CloseTerminal(id: string): Promise<void>
  DetectEnvironment(): Promise<Detection[]>
  ExitApp(): Promise<void>
  OpenInWindowsTerminal(dir: string): Promise<void>
}
export interface Runtime {
  EventsOn(name: 'terminal:output', callback: (payload: Output) => void): () => void
  EventsOn(name: 'terminal:exit', callback: (payload: Exit) => void): () => void
  EventsOn(name: 'app:close-requested', callback: (runningTerminals: number) => void): () => void
}
export type Bridge = { app: Backend; runtime: Runtime }

declare global {
  interface Window {
    go?: { main?: { App?: Backend } }
    runtime?: Runtime
  }
}

export function getBridge(): Bridge | null {
  const app = window.go?.main?.App
  const runtime = window.runtime
  return app && runtime && typeof runtime.EventsOn === 'function' &&
    ['StartTerminal', 'ResumeSession', 'ListSessions', 'WriteTerminal', 'ResizeTerminal', 'CloseTerminal', 'DetectEnvironment', 'ExitApp']
      .every(key => typeof app[key as keyof Backend] === 'function') ? { app, runtime } : null
}

export function decodeOutput(data: string): Uint8Array {
  return Uint8Array.from(atob(data), char => char.charCodeAt(0))
}
export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
