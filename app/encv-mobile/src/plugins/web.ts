import { WebPlugin } from '@capacitor/core'

export interface GoProcessStatus {
  running: boolean
  port: number
  lastError?: string
}

export interface GoProcessResult {
  success: boolean
  port?: number
  lastError?: string
}

export interface PermissionResult {
  granted: boolean
  requiresSettings?: boolean
}

export interface PermissionCheckResult {
  notifications: boolean
  storage: boolean
  batteryOptimization: boolean
}

export interface GoProcessPlugin {
  restart(): Promise<GoProcessResult>
  stop(): Promise<GoProcessResult>
  getStatus(): Promise<GoProcessStatus>
  requestNotificationPermission(): Promise<PermissionResult>
  requestStoragePermission(): Promise<PermissionResult>
  requestBatteryOptimization(): Promise<PermissionResult>
  checkPermissions(): Promise<PermissionCheckResult>
  isStandaloneMode(): Promise<{ standalone: boolean }>
  getIntentFileInfo(): Promise<{ path: string; name: string; mimeType: string }>
  openPlayer(options: { filePath: string; name: string; mimeType: string }): Promise<void>
  closePlayer(): Promise<void>
  openExternal(options: { url: string; mimeType: string }): Promise<void>
  openInPlayer(options: { path: string; name: string; mimeType: string }): Promise<void>
  openPlayerHome(): Promise<void>
  setScreenOrientation(options: { orientation: string }): Promise<void>
}

export class GoProcessWeb extends WebPlugin implements GoProcessPlugin {
  async restart(): Promise<GoProcessResult> {
    return { success: false }
  }

  async stop(): Promise<GoProcessResult> {
    return { success: false }
  }

  async getStatus(): Promise<GoProcessStatus> {
    return { running: false, port: 0 }
  }

  async requestNotificationPermission(): Promise<PermissionResult> {
    return { granted: true }
  }

  async requestStoragePermission(): Promise<PermissionResult> {
    return { granted: true }
  }

  async requestBatteryOptimization(): Promise<PermissionResult> {
    return { granted: true }
  }

  async checkPermissions(): Promise<PermissionCheckResult> {
    return { notifications: true, storage: true, batteryOptimization: true }
  }

  async isStandaloneMode(): Promise<{ standalone: boolean }> {
    return { standalone: false }
  }

  async getIntentFileInfo(): Promise<{ path: string; name: string; mimeType: string }> {
    return { path: '', name: '', mimeType: '' }
  }

  async openPlayer(_options: { filePath: string; name: string; mimeType: string }): Promise<void> {
  }

  async closePlayer(): Promise<void> {
  }

  async openExternal(_options: { url: string; mimeType: string }): Promise<void> {
  }

  async openInPlayer(_options: { path: string; name: string; mimeType: string }): Promise<void> {
  }

  async openPlayerHome(): Promise<void> {
  }

  async setScreenOrientation(_options: { orientation: string }): Promise<void> {
  }
}
