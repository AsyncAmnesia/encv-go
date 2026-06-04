/**
 * 共享 Vue 组件包
 *
 * 主 app 和 plugin-openlist/web 都引用此包：
 *   import { OpenListStatusCard, OpenListLogList } from '@encvgo/components'
 *
 * 组件通过 JS-Native 桥接（window.OpenListNative）与 plugin-openlist 通信，
 * 不依赖主 app 的 IPC 桥接。
 */
export { default as OpenListStatusCard } from './OpenListStatusCard.vue'
export { default as OpenListLogList } from './OpenListLogList.vue'

export interface OpenListRuntime {
  running: boolean
  port: number
  pid: number
  dataSizeBytes: number
  lastError: string
  lastUpdateTs: number
  dataDir: string
  isInstalled: boolean
}

export interface OpenListLog {
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
  timestamp: number
}
