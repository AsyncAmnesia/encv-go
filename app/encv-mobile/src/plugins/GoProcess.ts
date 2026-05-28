import { registerPlugin } from '@capacitor/core'

export type {
  GoProcessStatus,
  GoProcessResult,
  PermissionResult,
  PermissionCheckResult,
  GoProcessPlugin
} from './web'

import type { GoProcessPlugin, GoProcessResult, GoProcessStatus, PermissionResult, PermissionCheckResult } from './web'

const GoProcess = registerPlugin<GoProcessPlugin>('GoProcess', {
  web: () => import('./web').then(m => new m.GoProcessWeb()),
})

export function isNative(): boolean {
  return typeof window !== 'undefined' &&
    !!(window as any).Capacitor &&
    (window as any).Capacitor.isNativePlatform()
}

export async function restartBackend(): Promise<GoProcessResult> {
  try {
    return await GoProcess.restart()
  } catch (e: any) {
    console.error('[ENCV] GoProcess.restart() failed:', e?.message || e)
    return { success: false, lastError: e?.message || String(e) }
  }
}

export async function stopBackend(): Promise<GoProcessResult> {
  try {
    return await GoProcess.stop()
  } catch (e) {
    console.error('[ENCV] GoProcess.stop() failed:', e)
    return { success: false, lastError: e instanceof Error ? e.message : String(e) }
  }
}

export async function getBackendStatus(): Promise<GoProcessStatus> {
  try {
    return await GoProcess.getStatus()
  } catch (e) {
    console.error('[ENCV] GoProcess.getStatus() failed:', e)
    return { running: false, port: 0 }
  }
}

export async function requestNotificationPermission(): Promise<PermissionResult> {
  try {
    return await GoProcess.requestNotificationPermission()
  } catch (e) {
    console.error('[ENCV] GoProcess.requestNotificationPermission() failed:', e)
    return { granted: false }
  }
}

export async function requestStoragePermission(): Promise<PermissionResult> {
  try {
    return await GoProcess.requestStoragePermission()
  } catch (e) {
    console.error('[ENCV] GoProcess.requestStoragePermission() failed:', e)
    return { granted: false }
  }
}

export async function requestBatteryOptimization(): Promise<PermissionResult> {
  try {
    return await GoProcess.requestBatteryOptimization()
  } catch (e) {
    console.error('[ENCV] GoProcess.requestBatteryOptimization() failed:', e)
    return { granted: false }
  }
}

export async function checkPermissions(): Promise<PermissionCheckResult> {
  try {
    return await GoProcess.checkPermissions()
  } catch (e) {
    console.error('[ENCV] GoProcess.checkPermissions() failed:', e)
    return { notifications: false, storage: false, batteryOptimization: false }
  }
}

export async function isStandaloneMode(): Promise<{ standalone: boolean }> {
  try {
    return await GoProcess.isStandaloneMode()
  } catch (e) {
    console.error('[ENCV] GoProcess.isStandaloneMode() failed:', e)
    return { standalone: false }
  }
}

export async function getIntentFileInfo(): Promise<{ path: string; name: string; mimeType: string }> {
  try {
    return await GoProcess.getIntentFileInfo()
  } catch (e) {
    console.error('[ENCV] GoProcess.getIntentFileInfo() failed:', e)
    return { path: '', name: '', mimeType: '' }
  }
}

export async function openPlayer(filePath: string, name: string, mimeType: string): Promise<void> {
  try {
    await GoProcess.openPlayer({ filePath, name, mimeType })
  } catch (e) {
    console.error('[ENCV] GoProcess.openPlayer() failed:', e)
  }
}

export async function closePlayer(): Promise<void> {
  try {
    await GoProcess.closePlayer()
  } catch (e) {
    console.error('[ENCV] GoProcess.closePlayer() failed:', e)
  }
}

export async function openExternal(url: string, mimeType: string): Promise<void> {
  try {
    await GoProcess.openExternal({ url, mimeType })
  } catch (e) {
    console.error('[ENCV] GoProcess.openExternal() failed:', e)
  }
}

export async function openInPlayer(path: string, name: string, mimeType: string): Promise<void> {
  try {
    await GoProcess.openInPlayer({ path, name, mimeType })
  } catch (e) {
    console.error('[ENCV] GoProcess.openInPlayer() failed:', e)
  }
}

export async function openPlayerHome(): Promise<void> {
  try {
    await GoProcess.openPlayerHome()
  } catch (e) {
    console.error('[ENCV] GoProcess.openPlayerHome() failed:', e)
  }
}

export async function installExtensionApk(apkPath: string): Promise<{ success: boolean; method?: string }> {
  try {
    return await GoProcess.installPlugin({ apkPath })
  } catch (e) {
    console.error('[ENCV] GoProcess.installPlugin() failed:', e)
    return { success: false }
  }
}

export interface PickAndInstallResult {
  success: boolean
  method?: string
  fileName?: string
  pending?: boolean
}

export async function pickAndInstallPlugin(): Promise<PickAndInstallResult> {
  try {
    return await GoProcess.pickAndInstallPlugin()
  } catch (e) {
    console.error('[ENCV] GoProcess.pickAndInstallPlugin() failed:', e)
    return { success: false }
  }
}

export async function checkInstalledPlugins(): Promise<Record<string, boolean>> {
  try {
    const result = await GoProcess.checkInstalledPlugins()
    return result as Record<string, boolean>
  } catch (e) {
    console.error('[ENCV] GoProcess.checkInstalledPlugins() failed:', e)
    return {}
  }
}

export async function getLocalFilePath(path: string): Promise<string> {
  try {
    const result = await GoProcess.getLocalFilePath({ path })
    return result.path as string || ''
  } catch (e) {
    console.error('[ENCV] GoProcess.getLocalFilePath() failed:', e)
    return ''
  }
}
