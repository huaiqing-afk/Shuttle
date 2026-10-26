export type InventorySource = { Name: string; Count: number; Available: boolean; Error: string }
export type InventoryItem = {
  Name: string; Version: string; Source: string; Root: string; Ownership: string; Reason: string
}
export type InventoryReport = { Items: InventoryItem[] | null; Sources: InventorySource[] | null; Note: string }
export type InventoryPaths = { Ledger: string }
export type InventoryStep = {
  Item: InventoryItem
  // install steps that were not detected at all carry only a Package
  Package?: { Name: string; Version: string; Source: string; Root: string }
  Action: string
  // shown verbatim before any automatic install (contract §10.10 D8)
  Command?: string
  Instruction?: string
}
export type InstallPreview = { Token: string; Plan: InventoryPlan }
export type RemovalPreview = { Token: string; Plan: InventoryPlan }
export type InventoryRecord = {
  Name: string; Source: string; Version: string; State: string; OperationID: string; InstallerError: string
}
export type InstallResult = {
  Token: string; OperationID: string; State: string; Records: InventoryRecord[] | null; Error?: string
}
export type InventoryPlan = { Steps: InventoryStep[] | null; Blocked: boolean; Conflicts: string[] | null }
