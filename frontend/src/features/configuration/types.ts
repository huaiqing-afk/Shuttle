export type ConfigKey = { Keys: string; Command: string; Value: string }
export type ConfigChoice = {
  SchemaVersion: number
  WT: {
    Theme: string
    FontFace: string
    DefaultProfileGUID: string
    ManagedProfile: { GUID: string; Payload: string }
    Keys: ConfigKey[]
    Params: Record<string, string>
  }
  PS: { SessionEntry: boolean }
}
export type ConfigPaths = { WT: string; PowerShell: string; Operations: string }
export type ConfigDiff = { Target: string; Action: string; Before: string; After: string; WasAbsent: boolean; Conflict: string }
export type ConfigPlan = { PlanID: string; Diffs: ConfigDiff[] | null; Blocked: boolean }
export type ConfigStep = { Target: string; Action: string; Changed: boolean; Skipped: boolean; Err: string }
export type ConfigResult = { OpID: string; Steps: ConfigStep[] | null; Succeeded: boolean }
export type ConfigOperation = { ID: string; State: string; StartedAt: string; Steps: ConfigStep[] | null }
