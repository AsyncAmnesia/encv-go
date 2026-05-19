import { registerPlugin } from '@capacitor/core'

export interface GoProcessStatus {
  running: boolean
  port: number
}

export interface GoProcessResult {
  success: boolean
  port?: number
}

interface GoProcessPlugin {
  restart(): Promise<GoProcessResult>
  stop(): Promise<GoProcessResult>
  getStatus(): Promise<GoProcessStatus>
}

const GoProcess = registerPlugin<GoProcessPlugin>('GoProcess')

export function isNative(): boolean {
  return typeof window !== 'undefined' &&
    !!(window as any).Capacitor &&
    (window as any).Capacitor.isNativePlatform()
}

export async function restartBackend(): Promise<GoProcessResult> {
  if (!isNative()) {
    return { success: false }
  }
  return await GoProcess.restart()
}

export async function stopBackend(): Promise<GoProcessResult> {
  if (!isNative()) {
    return { success: false }
  }
  return await GoProcess.stop()
}

export async function getBackendStatus(): Promise<GoProcessStatus> {
  if (!isNative()) {
    return { running: false, port: 0 }
  }
  return await GoProcess.getStatus()
}
