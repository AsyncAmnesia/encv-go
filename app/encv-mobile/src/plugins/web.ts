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

export class GoProcessWeb implements GoProcessPlugin {
  async restart(): Promise<GoProcessResult> {
    console.warn('GoProcess.restart() called on web')
    return { success: false }
  }

  async stop(): Promise<GoProcessResult> {
    console.warn('GoProcess.stop() called on web')
    return { success: false }
  }

  async getStatus(): Promise<GoProcessStatus> {
    console.warn('GoProcess.getStatus() called on web')
    return { running: false, port: 0 }
  }

  async requestNotificationPermission(): Promise<PermissionResult> {
    console.warn('GoProcess.requestNotificationPermission() called on web')
    return { granted: true }
  }

  async requestStoragePermission(): Promise<PermissionResult> {
    console.warn('GoProcess.requestStoragePermission() called on web')
    return { granted: true }
  }

  async checkPermissions(): Promise<PermissionCheckResult> {
    console.warn('GoProcess.checkPermissions() called on web')
    return { notifications: true, storage: true }
  }
}
