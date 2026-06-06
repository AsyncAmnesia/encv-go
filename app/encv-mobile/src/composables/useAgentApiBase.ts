/**
 * useAgentApiBase - Agent API 基础 URL 解析
 *
 * Agent 端点（/api/encrypt-key、/api/decrypt-key、/api/chat 等）都挂载在
 * encv-go (Go 主后端) 的根路径下。三种环境下需要不同拼装：
 *
 *   ① Vite dev (web):
 *      - Vite :8100 不做反向代理（D9 决策）
 *      - 由 preview-gateway :16666 统一接管 /agent-api/* 转发到 encv-go :2025
 *      - 路径前缀必须是 '/agent-api'（带前缀的相对路径）
 *
 *   ② Capacitor APK (production):
 *      - 没有 preview-gateway，没有 vite proxy
 *      - encv-go 在设备本地 127.0.0.1:2025 上跑（useServerStatus 启动）
 *      - 直接绝对 URL 打到 encv-go 根路径
 *
 *   ③ Web SPA 静态托管:
 *      - 同 APK：走绝对 URL（getApiBaseUrl() 默认 127.0.0.1:2025）
 *
 * 调用方约定：
 *   fetch(`${getAgentApiBase()}/api/encrypt-key`, ...)
 *   fetch(`${getAgentApiBase()}/api/decrypt-key`, ...)
 *   fetch(`${getAgentApiBase()}/api/chat`,        ...)
 *   fetch(`${getAgentApiBase()}/test`,            ...)
 *
 * 行为表（与 dev / preview-gateway / APK 三态穷举验证）：
 *   import.meta.env.DEV = true            → '/agent-api'           (vite dev, 走网关)
 *   import.meta.env.DEV = false, native   → 'http://127.0.0.1:2025' (APK, 直连)
 *   import.meta.env.DEV = false, web SPA  → 用户配置 / DEFAULT_API_BASE_URL
 */

import { getApiBaseUrl, DEFAULT_API_BASE_URL } from '@/api/encv'
import { isNative } from '@/plugins/GoProcess'

/**
 * 解析 Agent API 基础 URL（同步）
 *
 * 注意：本函数只读 env + localStorage + isNative()，无副作用。
 * 任何需要根据后端状态动态调整的逻辑都不应放这里。
 */
export function getAgentApiBase(): string {
  if (import.meta.env.DEV) {
    // dev: 走 preview-gateway 统一前缀
    return '/agent-api'
  }
  // prod: APK 走 native bridge → 本地 :2025；web SPA 走 getApiBaseUrl() 配置
  // 两者都用绝对 URL，因为没有 vite proxy / preview-gateway 中转
  return getApiBaseUrl() || DEFAULT_API_BASE_URL
}

/**
 * 解析 Agent API base 的详细上下文（用于错误日志、DevLogs、状态徽标）
 * 让排错时一眼看清"当前 agent API 实际打到哪里"。
 */
export interface AgentApiBaseContext {
  base: string
  source: 'dev-gateway' | 'native-default' | 'user-configured' | 'web-fallback'
  isNative: boolean
  env: 'dev' | 'prod'
  /** 完整 URL 拼接样例（如 `${base}/api/encrypt-key`），便于日志对比 */
  sampleUrl: string
}

export function getAgentApiBaseContext(): AgentApiBaseContext {
  const env = import.meta.env.DEV ? 'dev' : 'prod'
  const native = isNative()

  if (env === 'dev') {
    return {
      base: '/agent-api',
      source: 'dev-gateway',
      isNative: native,
      env,
      sampleUrl: `${location.origin}/agent-api/api/encrypt-key`,
    }
  }

  // prod
  const apiBaseUrl = getApiBaseUrl()
  const hasUserOverride = (() => {
    try {
      return !!localStorage.getItem('encv-server-url')
    } catch {
      return false
    }
  })()
  const source: AgentApiBaseContext['source'] = native
    ? 'native-default'
    : hasUserOverride
      ? 'user-configured'
      : 'web-fallback'

  return {
    base: apiBaseUrl || DEFAULT_API_BASE_URL,
    source,
    isNative: native,
    env,
    sampleUrl: `${apiBaseUrl || DEFAULT_API_BASE_URL}/api/encrypt-key`,
  }
}
