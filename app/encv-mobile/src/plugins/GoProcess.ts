import { registerPlugin } from '@capacitor/core'

export type {
  GoProcessStatus,
  GoProcessResult,
  PermissionResult,
  PermissionCheckResult,
  GoProcessPlugin,
  PluginFullState
} from './web'

import type { GoProcessPlugin, GoProcessResult, GoProcessStatus, PermissionResult, PermissionCheckResult, PluginFullState } from './web'

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

export interface PlayResult {
  success: boolean
  error?: string
  errorDetail?: string
}

export async function openPlayer(filePath: string, name: string, mimeType: string, mode?: string): Promise<PlayResult> {
  try {
    const result = await GoProcess.openPlayer({ filePath, name, mimeType, mode: mode || '' })
    if (result.success === false) {
      console.error('[ENCV] openPlayer failed:', result.error, result.errorDetail)
      return { success: false, error: result.error, errorDetail: result.errorDetail }
    }
    return { success: true }
  } catch (e) {
    console.error('[ENCV] GoProcess.openPlayer() failed:', e)
    return { success: false, error: '调用播放器失败', errorDetail: String(e) }
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

export async function openInPlayer(path: string, name: string, mimeType: string, mode?: string): Promise<void> {
  try {
    await GoProcess.openInPlayer({ path, name, mimeType, mode: mode || '' })
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
  error?: string
}

export async function pickAndInstallPlugin(): Promise<PickAndInstallResult> {
  try {
    return await GoProcess.pickAndInstallPlugin()
  } catch (e: any) {
    const msg = e?.message || e?.code || String(e)
    console.error('[ENCV] GoProcess.pickAndInstallPlugin() failed:', msg)
    return { success: false, error: msg }
  }
}

export async function checkInstalledPlugins(): Promise<Record<string, { installed: boolean; enabled: boolean; versionName: string }>> {
  try {
    const result = await GoProcess.checkInstalledPlugins()
    return result
  } catch (e) {
    console.error('[ENCV] GoProcess.checkInstalledPlugins() failed:', e)
    return {}
  }
}

export async function togglePluginEnabled(pluginId: string, enabled: boolean): Promise<{ success: boolean; pluginId: string; enabled: boolean }> {
  try {
    return await GoProcess.togglePluginEnabled({ pluginId, enabled })
  } catch (e: any) {
    console.error('[ENCV] GoProcess.togglePluginEnabled() failed:', e?.message || e)
    return { success: false, pluginId, enabled }
  }
}

export async function uninstallPlugin(pluginId: string): Promise<{ success: boolean; pluginId: string }> {
  try {
    return await GoProcess.uninstallPlugin({ pluginId })
  } catch (e: any) {
    console.error('[ENCV] GoProcess.uninstallPlugin() failed:', e?.message || e)
    return { success: false, pluginId }
  }
}

export async function debugLifecycleFlow(pluginId?: string): Promise<Record<string, any>> {
  try {
    return await GoProcess.debugLifecycleFlow({ pluginId })
  } catch (e) {
    console.error('[ENCV] GoProcess.debugLifecycleFlow() failed:', e)
    return { error: e instanceof Error ? e.message : String(e) }
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

export async function debugInstallFlow(): Promise<Record<string, any>> {
  try {
    return await GoProcess.debugInstallFlow()
  } catch (e) {
    console.error('[ENCV] GoProcess.debugInstallFlow() failed:', e)
    return { error: e instanceof Error ? e.message : String(e) }
  }
}

export async function debugKotlinReflect(): Promise<Record<string, any>> {
  try {
    return await GoProcess.debugKotlinReflect()
  } catch (e) {
    console.error('[ENCV] GoProcess.debugKotlinReflect() failed:', e)
    return { error: e instanceof Error ? e.message : String(e) }
  }
}

export async function debugApkValidation(): Promise<Record<string, any>> {
  try {
    return await GoProcess.debugApkValidation()
  } catch (e) {
    console.error('[ENCV] GoProcess.debugApkValidation() failed:', e)
    return { error: e instanceof Error ? e.message : String(e) }
  }
}

export async function debugValidationStrategy(): Promise<Record<string, any>> {
  try {
    return await GoProcess.debugValidationStrategy()
  } catch (e) {
    console.error('[ENCV] GoProcess.debugValidationStrategy() failed:', e)
    return { error: e instanceof Error ? e.message : String(e) }
  }
}

export async function exportLogs(): Promise<{ success: boolean; path?: string }> {
  try {
    return await GoProcess.exportLogs()
  } catch (e) {
    console.error('[ENCV] GoProcess.exportLogs() failed:', e)
    return { success: false }
  }
}

export async function clearLogs(): Promise<{ success: boolean }> {
  try {
    return await GoProcess.clearLogs()
  } catch (e) {
    console.error('[ENCV] GoProcess.clearLogs() failed:', e)
    return { success: false }
  }
}

export async function openLogViewer(): Promise<{ success: boolean }> {
  try {
    return await GoProcess.openLogViewer()
  } catch (e) {
    console.error('[ENCV] GoProcess.openLogViewer() failed:', e)
    return { success: false }
  }
}

export async function saveDevLogs(logs: string): Promise<{ success: boolean; path?: string }> {
  try {
    return await GoProcess.saveDevLogs({ logs })
  } catch (e) {
    console.error('[ENCV] GoProcess.saveDevLogs() failed:', e)
    return { success: false }
  }
}

export async function getPluginFullState(pluginId: string): Promise<PluginFullState> {
  try {
    const result = await GoProcess.getPluginFullState({ pluginId })
    return result
  } catch (e) {
    console.error('[GoProcess] getPluginFullState failed:', e)
    return { id: pluginId, status: 'error', name: '', version: '' }
  }
}

export async function ensurePluginLoaded(pluginId: string): Promise<boolean> {
  try {
    const result = await GoProcess.ensurePluginLoaded({ pluginId })
    return result.success === true
  } catch (e) {
    console.error('[GoProcess] ensurePluginLoaded failed:', e)
    return false
  }
}

export async function startMpvInPlace(filePath: string, fileName: string, mimeType?: string): Promise<PlayResult> {
  try {
    const result = await GoProcess.startMpvInPlace({ filePath, name: fileName, mimeType: mimeType || '' })
    if (result.success === false) {
      return { success: false, error: result.error, errorDetail: result.errorDetail }
    }
    return { success: true }
  } catch (e) {
    console.error('[GoProcess] startMpvInPlace failed:', e)
    return { success: false, error: '嵌入播放器启动失败', errorDetail: String(e) }
  }
}

export async function stopMpvInPlace(): Promise<{ success: boolean }> {
  try {
    const result = await GoProcess.stopMpvInPlace()
    return { success: result.success === true }
  } catch (e) {
    console.error('[GoProcess] stopMpvInPlace failed:', e)
    return { success: false }
  }
}
