/**
 * useChatEngine.ts - 运行时引擎切换器
 *
 * 管理当前活跃的 ChatEngine 实例，支持：
 * - localStorage 持久化引擎选择
 * - reactive 响应式切换（无需刷新页面）
 * - 自动 fallback 到 default 引擎
 * - destroy/实例化生命周期管理
 *
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/tasks.md Task 1.2
 */

import { shallowRef, type ShallowRef } from 'vue'
import type { ChatEngine } from './chatEngine'
import {
  createEngineInstance,
  getRegisteredEngines,
} from './chatEngine'

// =============================================================================
// 常量
// =============================================================================

const STORAGE_KEY = 'encv-chat-engine'
const DEFAULT_ENGINE_ID = 'default'

// =============================================================================
// 引擎状态（模块级单例）
// =============================================================================

/** 当前活跃引擎实例 */
let currentEngineInstance: ChatEngine | null = null
/** 当前引擎 ID */
let currentEngineId: string = loadSavedEngineId()

/**
 * 从 localStorage 加载保存的引擎 ID
 * 如果保存的值无效或引擎不存在，返回 DEFAULT_ENGINE_ID
 */
function loadSavedEngineId(): string {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && typeof saved === 'string' && saved.trim().length > 0) {
      return saved.trim()
    }
  } catch {
    // localStorage 可能不可用（SSR / 隐私模式）
  }
  return DEFAULT_ENGINE_ID
}

/** 持久化引擎选择 */
function saveEngineId(id: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, id)
  } catch {
    // 静默失败
  }
}

// =============================================================================
// useChatEngine Composable
// =============================================================================

export interface UseChatEngineReturn {
  /** 当前活跃引擎（shallowRef，切换时触发重新渲染） */
  currentEngine: ShallowRef<ChatEngine | null>
  /** 当前引擎 ID */
  currentEngineId: string
  /** 所有已注册引擎的元信息列表 */
  engineList: Array<{ id: string; name: string; description?: string }>
  /**
   * 切换到指定引擎
   * @param id 目标引擎 id
   * @returns 是否切换成功
   */
  switchEngine: (id: string) => boolean
}

/**
 * 获取引擎切换器实例
 *
 * 使用方式：
 * ```vue
 * <script setup>
 * const { currentEngine, switchEngine, engineList } = useChatEngine()
 * </script>
 * <template>
 *   <component :is="currentEngine?.renderMessages(props)" />
 *   <!-- 切换按钮 -->
 *   <select @change="switchEngine($event.target.value)">
 *     <option v-for="e in engineList" :key="e.id" :value="e.id" :selected="e.id === currentEngineId">
 *       {{ e.name }}
 *     </option>
 *   </select>
 * </template>
 * ```
 */
export function useChatEngine(): UseChatEngineReturn {
  const currentEngine = shallowRef<ChatEngine | null>(currentEngineInstance)

  // 确保有活跃实例
  if (!currentEngine.value) {
    currentEngine.value = ensureEngine(currentEngineId)
  }

  function switchEngine(id: string): boolean {
    if (id === currentEngineId && currentEngine.value) {
      return true // 已经是目标引擎
    }

    // 销毁旧引擎
    if (currentEngine.value) {
      try {
        currentEngine.value.destroy()
      } catch (err) {
        console.warn(`[useChatEngine] Error destroying previous engine`, err)
      }
    }

    // 创建新引擎
    const newEngine = ensureEngine(id)
    if (!newEngine) {
      // fallback：如果目标引擎创建失败，回退到 default
      console.warn(`[useChatEngine] Engine "${id}" failed to create, falling back to default`)
      const fallback = ensureEngine(DEFAULT_ENGINE_ID)
      currentEngine.value = fallback
      currentEngineId = DEFAULT_ENGINE_ID
      saveEngineId(DEFAULT_ENGINE_ID)
      return false
    }

    currentEngine.value = newEngine
    currentEngineId = id
    saveEngineId(id)
    return true
  }

  return {
    currentEngine,
    currentEngineId,
    engineList: getRegisteredEngines(),
    switchEngine,
  }
}

/**
 * 确保引擎可用 —— 如果实例不存在则创建，失败则尝试 default
 */
function ensureEngine(id: string): ChatEngine | null {
  let instance = createEngineInstance(id)
  if (!instance && id !== DEFAULT_ENGINE_ID) {
    console.warn(`[useChatEngine] Engine "${id}" not found, falling back to default`)
    instance = createEngineInstance(DEFAULT_ENGINE_ID)
    if (instance) {
      currentEngineId = DEFAULT_ENGINE_ID
      saveEngineId(DEFAULT_ENGINE_ID)
    }
  }
  return instance
}

/** 导出常量供外部使用 */
export { DEFAULT_ENGINE_ID, STORAGE_KEY }
