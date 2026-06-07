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

import { shallowRef, ref, type ShallowRef, type Ref } from 'vue'
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
// 引擎状态（模块级单例，响应式）
// =============================================================================

/**
 * 当前活跃引擎实例（shallowRef，切换时触发重新渲染）
 * 模块级单例——所有 useChatEngine() 调用共享同一个 ref
 */
const currentEngine: ShallowRef<ChatEngine | null> = shallowRef(null)

/** 当前引擎 ID（响应式，用于 select 绑定） */
const activeEngineId = ref(loadSavedEngineId())

/** 所有已注册引擎的元信息列表（响应式） */
const engineList = ref(getRegisteredEngines())

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

/**
 * 确保引擎可用 —— 如果实例不存在则创建，失败则尝试 default
 */
function ensureEngine(id: string): ChatEngine | null {
  let instance = createEngineInstance(id)
  if (!instance && id !== DEFAULT_ENGINE_ID) {
    console.warn(`[useChatEngine] Engine "${id}" not found, falling back to default`)
    instance = createEngineInstance(DEFAULT_ENGINE_ID)
    if (instance) {
      activeEngineId.value = DEFAULT_ENGINE_ID
      saveEngineId(DEFAULT_ENGINE_ID)
    }
  }
  return instance
}

// 初始化：确保有活跃实例
if (!currentEngine.value) {
  currentEngine.value = ensureEngine(activeEngineId.value)
  if (currentEngine.value) {
    activeEngineId.value = currentEngine.value.id
  }
}

// =============================================================================
// useChatEngine Composable
// =============================================================================

export interface UseChatEngineReturn {
  /** 当前活跃引擎（shallowRef，切换时触发重新渲染） */
  currentEngine: ShallowRef<ChatEngine | null>
  /** 当前引擎 ID（响应式，用于 select 绑定） */
  currentEngineId: Ref<string>
  /** 所有已注册引擎的元信息列表（响应式） */
  engineList: Ref<Array<{ id: string; name: string; description?: string }>>
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
 * 所有调用方共享同一个模块级响应式状态（currentEngine / activeEngineId / engineList），
 * 切换引擎时所有使用方自动更新。
 */
export function useChatEngine(): UseChatEngineReturn {
  function switchEngine(id: string): boolean {
    if (id === activeEngineId.value && currentEngine.value) {
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
      activeEngineId.value = DEFAULT_ENGINE_ID
      saveEngineId(DEFAULT_ENGINE_ID)
      return false
    }

    currentEngine.value = newEngine
    activeEngineId.value = id
    saveEngineId(id)

    // 刷新引擎列表（可能有新引擎注册）
    engineList.value = getRegisteredEngines()

    return true
  }

  return {
    currentEngine,
    currentEngineId: activeEngineId,
    engineList,
    switchEngine,
  }
}

/** 导出常量供外部使用 */
export { DEFAULT_ENGINE_ID, STORAGE_KEY }
