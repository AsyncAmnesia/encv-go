import { WebPlugin } from '@capacitor/core'

export interface GoProcessStatus {
  running: boolean
  port: number
}

export interface GoProcessResult {
  success: boolean
  port?: number
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
}
