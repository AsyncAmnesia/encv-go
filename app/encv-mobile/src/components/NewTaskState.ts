import type { PluginCandidate, ContainerVersionInfo, TaskField, TaskOptions } from '@/api/encv'

export interface NewTaskState {
  taskType: string
  sourcePath: string
  targetPath: string
  candidates: PluginCandidate[]
  predictedPlugin: string | null
  taskOptions: TaskOptions | null
  primaryOverride: string
  secondaryPassword: string
  version: number
  versionOptions: ContainerVersionInfo[]
  extraValues: Record<string, string>
  filteredExtraFields: TaskField[]
  selectedPluginIndex: number
}
