import type { ConfigChoice, ConfigOperation, ConfigPaths, ConfigPlan, ConfigResult } from '../features/configuration/types'

export interface ConfigurationBackend {
 ConfigurationPaths(): Promise<ConfigPaths>
 PlanConfiguration(choice: ConfigChoice): Promise<ConfigPlan>
 ExecuteConfiguration(id: string): Promise<ConfigResult>
 ListConfigurationOperations(): Promise<ConfigOperation[]>
 PlanConfigurationRestore(id: string): Promise<ConfigPlan>
}
