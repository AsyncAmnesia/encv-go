import { registerPlugin } from '@capacitor/core'

// 从 web.ts 重新导出类型，避免重复定义
export type {
  GoProcessStatus,
  GoProcessResult,
  PermissionResult,
  PermissionCheckResult,
  GoProcessPlugin
} from './web'

import { GoProcessWeb } from './web'

// 安全的插件注册，避免在非原生平台出错
let GoProcess: GoProcessPlugin | null = null

try {
  GoProcess = registerPlugin<GoProcessPlugin>('GoProcess', {
    web: () => Promise.resolve({ webPlugin: () => new GoProcessWeb() }),
  })
} catch (e) {
  console.warn('GoProcess plugin registration failed:', e)
}

export function isNative(): boolean {
  return typeof window !== 'undefined' &&
    !!(window as any).Capacitor &&
    (window as any).Capacitor.isNativePlatform()
}

export async function restartBackend(): Promise<GoProcessResult> {
  if (!isNative() || !GoProcess) {
    console.warn('restartBackend called on non-native platform')
    return { success: false }
  }
  try {
    console.log('Calling GoProcess.restart()...')
    const result = await GoProcess.restart()
    console.log('GoProcess.restart() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.restart() failed:', e)
    return { success: false }
  }
}

export async function stopBackend(): Promise<GoProcessResult> {
  if (!isNative() || !GoProcess) {
    console.warn('stopBackend called on non-native platform')
    return { success: false }
  }
  try {
    console.log('Calling GoProcess.stop()...')
    const result = await GoProcess.stop()
    console.log('GoProcess.stop() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.stop() failed:', e)
    return { success: false }
  }
}

export async function getBackendStatus(): Promise<GoProcessStatus> {
  if (!isNative() || !GoProcess) {
    console.warn('getBackendStatus called on non-native platform')
    return { running: false, port: 0 }
  }
  try {
    console.log('Calling GoProcess.getStatus()...')
    const result = await GoProcess.getStatus()
    console.log('GoProcess.getStatus() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.getStatus() failed:', e)
    return { running: false, port: 0 }
  }
}

export async function requestNotificationPermission(): Promise<PermissionResult> {
  if (!isNative() || !GoProcess) {
    console.warn('requestNotificationPermission called on non-native platform')
    return { granted: true }
  }
  try {
    console.log('Calling GoProcess.requestNotificationPermission()...')
    const result = await GoProcess.requestNotificationPermission()
    console.log('GoProcess.requestNotificationPermission() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.requestNotificationPermission() failed:', e)
    return { granted: false }
  }
}

export async function requestStoragePermission(): Promise<PermissionResult> {
  if (!isNative() || !GoProcess) {
    console.warn('requestStoragePermission called on non-native platform')
    return { granted: true }
  }
  try {
    console.log('Calling GoProcess.requestStoragePermission()...')
    const result = await GoProcess.requestStoragePermission()
    console.log('GoProcess.requestStoragePermission() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.requestStoragePermission() failed:', e)
    return { granted: false }
  }
}

export async function checkPermissions(): Promise<PermissionCheckResult> {
  if (!isNative() || !GoProcess) {
    console.warn('checkPermissions called on non-native platform')
    return { notifications: true, storage: true }
  }
  try {
    console.log('Calling GoProcess.checkPermissions()...')
    const result = await GoProcess.checkPermissions()
    console.log('GoProcess.checkPermissions() result:', result)
    return result
  } catch (e) {
    console.error('GoProcess.checkPermissions() failed:', e)
    return { notifications: false, storage: false }
  }
}
