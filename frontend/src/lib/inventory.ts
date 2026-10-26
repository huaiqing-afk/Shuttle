import type {
  InstallPreview, InstallResult, InventoryPaths, InventoryRecord, InventoryReport, RemovalPreview,
} from '../features/inventory/types'

export interface InventoryBackend {
  InventoryPaths(): Promise<InventoryPaths>
  DetectInventory(): Promise<InventoryReport>
  PlanInventoryRemoval(names: string[]): Promise<RemovalPreview>
  ExecuteInventoryRemoval(previewToken: string): Promise<InstallResult>
  // A preview must be generated and shown before ExecuteInstall will run it.
  PlanInstall(componentIDs: string[]): Promise<InstallPreview>
  ExecuteInstall(previewToken: string): Promise<InstallResult>
  ReconcileInventory(): Promise<InventoryRecord[] | null>
}
