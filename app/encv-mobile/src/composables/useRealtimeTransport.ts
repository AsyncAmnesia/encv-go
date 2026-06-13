/**
 * useRealtimeTransport — 统一实时传输单例（2026-06-10 重构）
 *
 * 目标：
 *   - 集中 transport 模式选举（ws / http-poll / native-bridge）
 *   - 集中 fallback 逻辑（不再每个用到 WS 的地方写 isOpenPreviewBrowser）
 *   - 消费方继续 eventBus.on，不感知 transport 变化
 *   - 新增 WS 事件类型 → 只改 backend 实现，消费方零改动
 *
 * 用法：
 *   - App.vue: const transport = useRealtimeTransport(); transport.connect()
 *   - 消费方: eventBus.on('task:update', cb)（不变）
 *
 * 选举规则（按优先级）：
 *   1. _forcedMode（测试用强制模式）
 *   2. isNative()                → native-bridge
 *   3. isOpenPreviewBrowser()    → http-poll
 *   4. 默认                       → ws
 *
 * 历史 bug 根因：
 *   - 沙箱 OpenPreview 浏览器 trae 反代 :16000 不支持 WebSocket upgrade
 *   - 之前每个用到 WS 的地方都自己写 isOpenPreviewBrowser / isSandboxBrowser 判断
 *   - 散落在 useWebSocket.ts / useServerStatus.ts / useFrontendLogs.ts / DevLogs.vue
 *   - 加新 WS 事件类型 = 改 4+ 文件
 *
 * 验证：
 *   - ws 模式：沙箱本地 dev (localhost:16666) / 真机浏览器
 *   - http-poll 模式：OpenPreview 浏览器
 *   - native-bridge 模式：APK 真机（暂未实现）
 */

import { ref, type Ref } from 'vue'
import { isOpenPreviewBrowser, getApiBaseUrl } from '@/api/encv'
import { eventBus } from './useEventBus'
import { createWsBackend } from './realtime/WsBackend'
import { createHttpPollBackend } from './realtime/HttpPollBackend'
import { createNativeBridgeBackend } from './realtime/NativeBridgeBackend'
import type { Backend, ConnectionState } from './realtime/Backend'

export type TransportMode = 'ws' | 'http-poll' | 'native-bridge' | 'unknown'

export interface RealtimeTransport {
  /** 启动 transport（幂等：重复调用只触发一次） */
  connect(): void
  /** 停止 transport */
  disconnect(): void
  /** 强制重连（先 disconnect 再 connect） */
  forceReconnect(): void
  /** 当前连接状态 */
  readonly connectionState: Readonly<Ref<ConnectionState>>
  /** 当前 transport 模式（connect 后才会变） */
  readonly transportMode: Readonly<Ref<TransportMode>>
  /** 当前是否在 OpenPreview 浏览器（只读） */
  readonly isSandboxBrowser: Readonly<Ref<boolean>>
  /** 测试用：强制 transport 模式（传 null 恢复默认选举） */
  __forceMode(mode: TransportMode | null): void
  /** 测试用：重置单例（仅在测试 setup/teardown 用） */
  __resetForTesting(): void
}

// 模块级单例
let _instance: RealtimeTransport | null = null
let _forcedMode: TransportMode | null = null

/**
 * 检测是否在 Capacitor native 环境（APK / iOS）
 *
 * 关键决策（2026-06-11）：native 真机仍走 ws 模式，**不**用 native-bridge。
 *
 * 原因：
 *   1. NativeBridgeBackend 现状是空壳（start() 只 emit online:true，无事件转发）——
 *      走 native-bridge 意味着真机 task 进度永远不更新、mock 写盘永远不触发
 *   2. Capacitor WebView = Android System WebView / iOS WKWebView = Chrome/Safari 内核
 *      → WebSocket 原生支持，直连 127.0.0.1:2025 即可
 *   3. ws 模式比 native-bridge 简单得多（不用写 native plugin）
 *
 * native-bridge 留作未来 SSE / 设备本地 socket 实现（见 NativeBridgeBackend.ts TODO）。
 */
function isNative(): boolean {
  if (typeof window === 'undefined') return false
  // Capacitor 在 window 上挂 capacitor 全局对象
  const cap = (window as any).Capacitor
  return Boolean(cap && typeof cap.isNativePlatform === 'function' && cap.isNativePlatform())
}

/** 选举 transport 模式
 *
 * 顺序（2026-06-11 v3 修正）：
 *   1. _forcedMode（测试用强制模式）
 *   2. isOpenPreviewBrowser()    → http-poll（沙箱 trae 反代 :16000 不支持 WS upgrade）
 *   3. 默认                       → ws（**包括真机** Capacitor WebView，NativeBridgeBackend 暂未实现）
 *
 * 历史：原顺序是 isNative() → native-bridge（空壳）→ 真机进度永远不更新
 */
function electMode(): TransportMode {
  if (_forcedMode) return _forcedMode
  // 关键：OpenPreview 浏览器判断放在 isNative 之前
  // 因为 trae OpenPreview 也可能在 webview 内跑（Capacitor 检测可能误报）
  if (isOpenPreviewBrowser()) return 'http-poll'
  return 'ws'
}

/** baseUrl 变更监听（当 serverUrl 改变时强制重连） */
function ensureApiBaseListeners(transport: RealtimeTransport): () => void {
  if (typeof window === 'undefined') return () => {}
  const onApiBaseConnected = () => {
    console.info('[RealtimeTransport] api-base:connected → forceReconnect')
    setTimeout(() => transport.forceReconnect(), 100)
  }
  const onStorage = (e: StorageEvent) => {
    if (e.key !== 'encv-server-url') return
    if (!e.newValue) return
    console.info('[RealtimeTransport] storage event: encv-server-url changed → forceReconnect')
    setTimeout(() => transport.forceReconnect(), 100)
  }
  eventBus.on('api-base:connected', onApiBaseConnected)
  window.addEventListener('storage', onStorage)
  return () => {
    eventBus.off('api-base:connected', onApiBaseConnected)
    window.removeEventListener('storage', onStorage)
  }
}

function createTransport(): RealtimeTransport {
  const connectionState = ref<ConnectionState>('disconnected')
  const transportMode = ref<TransportMode>('unknown')
  const isSandboxBrowser = ref(isOpenPreviewBrowser())
  let backend: Backend | null = null
  let cleanupListeners: (() => void) | null = null

  function ensureBackend(): Backend {
    if (backend) return backend
    const mode = electMode()
    transportMode.value = mode
    const emit = (type: string, data: any) => eventBus.emit(type as any, data)
    switch (mode) {
      case 'native-bridge':
        backend = createNativeBridgeBackend(emit)
        break
      case 'http-poll':
        backend = createHttpPollBackend(emit, {
          onConnected: () => { connectionState.value = 'connected' },
        })
        break
      case 'ws':
        backend = createWsBackend(emit, {
          onConnected: () => { connectionState.value = 'connected' },
          onDisconnected: () => { connectionState.value = 'disconnected' },
        })
        break
    }
    return backend!
  }

  // 首次 connect 注册 baseUrl 监听
  function ensureListeners() {
    if (cleanupListeners) return
    cleanupListeners = ensureApiBaseListeners({
      connect: () => {}, disconnect: () => {}, forceReconnect: () => {},
      connectionState, transportMode, isSandboxBrowser,
      __forceMode: () => {}, __resetForTesting: () => {},
    })
  }

  return {
    connect() {
      if (connectionState.value === 'connecting' || connectionState.value === 'connected') return
      ensureListeners()
      connectionState.value = 'connecting'
      ensureBackend().start()
    },
    disconnect() {
      backend?.stop()
      backend = null
      connectionState.value = 'disconnected'
      transportMode.value = 'unknown'
    },
    forceReconnect() {
      backend?.stop()
      backend = null
      this.connect()
    },
    connectionState: connectionState as Readonly<Ref<ConnectionState>>,
    transportMode: transportMode as Readonly<Ref<TransportMode>>,
    isSandboxBrowser: isSandboxBrowser as Readonly<Ref<boolean>>,
    __forceMode(mode) {
      _forcedMode = mode
      backend?.stop()
      backend = null
      transportMode.value = 'unknown'
      connectionState.value = 'disconnected'
    },
    __resetForTesting() {
      backend?.stop()
      backend = null
      cleanupListeners?.()
      cleanupListeners = null
      transportMode.value = 'unknown'
      connectionState.value = 'disconnected'
      _instance = null
      _forcedMode = null
    },
  }
}

export function useRealtimeTransport(): RealtimeTransport {
  if (!_instance) _instance = createTransport()
  return _instance
}

/** 内部使用：test/console log 拿当前 transport mode（不影响生产逻辑） */
export function getActiveTransportMode(): TransportMode {
  return electMode()
}

/** 内部使用：debug 打印当前 baseUrl + transport mode */
export function getTransportDebugInfo(): { mode: TransportMode; baseUrl: string; isSandboxBrowser: boolean; isNative: boolean } {
  return {
    mode: electMode(),
    baseUrl: getApiBaseUrl(),
    isSandboxBrowser: isOpenPreviewBrowser(),
    isNative: isNative(),
  }
}
