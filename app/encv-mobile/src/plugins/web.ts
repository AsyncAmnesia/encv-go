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
}

export interface GoProcessPlugin {
  restart(): Promise<GoProcessResult>
  stop(): Promise<GoProcessResult>
  getStatus(): Promise<GoProcessStatus>
  requestNotificationPermission(): Promise<PermissionResult>
  requestStoragePermission(): Promise<PermissionResult>
  checkPermissions(): Promise<PermissionCheckResult>
  isStandaloneMode(): Promise<{ standalone: boolean }>
  getIntentFileInfo(): Promise<{ path: string; name: string; mimeType: string }>
  openInPlayer(options: { path: string; name: string; mimeType: string }): Promise<void>
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

  async checkPermissions(): Promise<PermissionCheckResult> {
    return { notifications: true, storage: true }
  }

  async isStandaloneMode(): Promise<{ standalone: boolean }> {
    return { standalone: false }
  }

  async getIntentFileInfo(): Promise<{ path: string; name: string; mimeType: string }> {
    return { path: '', name: '', mimeType: '' }
  }

  async openInPlayer(_options: { path: string; name: string; mimeType: string }): Promise<void> {
  }
}
