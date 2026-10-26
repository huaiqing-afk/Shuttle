import type { Agent } from '../../lib/bridge'
import type { Phase } from './session'

export type Tab = {
  key: number
  agent: Agent
  title: string
  /** set when the tab resumes a historical session; re-picking that session focuses this tab */
  sessionId?: string
  phase: Phase
}

export const agentNames: Record<Agent, string> = {
  powershell: 'PowerShell', claude: 'Claude Code', codex: 'Codex',
}
export const phaseLabels: Record<Phase, string> = {
  idle: '等待', starting: '正在启动', running: '运行中',
  stopping: '正在关闭', exited: '已退出', failed: '启动失败',
}
