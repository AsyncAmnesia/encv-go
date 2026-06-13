/**
 * dev-start-guard.test.ts
 *
 * 🆕 2026-06-10 CI build 误拦截防回归（用户报告 `pnpm run build` 失败）：
 *  验证 devStartGuard 在 build 模式 / CI 环境 / SPAWN_VITE=1 / PM2_HOME 存在时
 *  全部不抛错，只在"裸跑 vite dev + 无合法 env"时抛错。
 *
 * 文件位置说明：此文件在 src/composables/__tests__/（vue-tsc 扫描范围），
 *  但**不** import node:fs/path/url 等协议 — 故不会因 tsconfig 不加载
 *  @types/node 报 TS2307。
 *
 * 测试策略：通过 vi.stubEnv 修改 process.env 后用 devStartGuard().config()
 *  钩子（传不同 env.command 参数）验证行为。每个 it 完成后恢复 env，
 *  避免污染后续测试。
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { devStartGuard } from '@/lib/dev-start-guard'

// ⚠️ vite.config.ts 用相对路径 import src/lib/dev-start-guard.ts：
//   import { devStartGuard } from './src/lib/dev-start-guard'
// 测试文件在 src/composables/__tests__/，用 @ 别名指向 src/lib/，清晰一致

interface MinimalEnv {
  command: 'serve' | 'build'
  mode?: string
}

// Vite 8 Plugin['config'] 是 ObjectHook（{ handler, order? }），不是直接函数
// 需要 plugin.config.handler(config, env) 调用
type ConfigHandler = (this: unknown, config: Record<string, unknown>, env: MinimalEnv) => void | Record<string, unknown> | null

function callGuard(plugin: ReturnType<typeof devStartGuard>, env: MinimalEnv): void {
  const hook = plugin.config as unknown as { handler: ConfigHandler } | ConfigHandler
  const handler = typeof hook === 'function' ? hook : hook.handler
  handler.call(undefined, {}, env)
}

describe('devStartGuard — 触发条件防回归', () => {
  const ORIGINAL_ENV = { ...process.env }

  beforeEach(() => {
    // 清掉所有守卫关心的 env
    delete process.env.SPAWN_VITE
    delete process.env.PM2_HOME
    delete process.env.PPA_SPAWNED
    delete process.env.CI
  })

  afterEach(() => {
    // 恢复原始 env（避免污染其他测试文件）
    for (const k of Object.keys(process.env)) delete process.env[k]
    Object.assign(process.env, ORIGINAL_ENV)
  })

  // === build 模式必须跳过 ===
  it('build 模式 + 无合法 env → 不抛错（CI 产线打包能跑）', () => {
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'build' })).not.toThrow()
  })

  it('build 模式 + 显式 SPAWN_VITE/PM2_HOME 都为空 → 不抛错', () => {
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'build' })).not.toThrow()
  })

  // === CI 环境必须跳过 ===
  it('serve 模式 + CI=true + 无 PM2 → 不抛错（GitHub Actions 跑 lint）', () => {
    process.env.CI = 'true'
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })

  it('serve 模式 + CI=1 + 无 PM2 → 不抛错', () => {
    process.env.CI = '1'
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })

  // === SPAWN_VITE=1 必须放过 ===
  it('serve 模式 + SPAWN_VITE=1 → 不抛错（preview-gateway spawn 合法）', () => {
    process.env.SPAWN_VITE = '1'
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })

  // === PM2_HOME 存在必须放过 ===
  it('serve 模式 + PM2_HOME 有值 → 不抛错（PM2 在管 vite）', () => {
    process.env.PM2_HOME = '/root/.pm2'
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })

  // === PPA_SPAWNED 兼容老版本 ===
  it('serve 模式 + PPA_SPAWNED=1 → 不抛错（preview-gateway 老版本）', () => {
    process.env.PPA_SPAWNED = '1'
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })

  // === 必须抛错的场景 ===
  it('serve 模式 + 无任何合法 env → 抛错（裸跑 vite）', () => {
    const plugin = devStartGuard({ errorMessage: 'TASK_FAILED_TOKEN_裸跑vite' })
    expect(() => callGuard(plugin, { command: 'serve' })).toThrow(/TASK_FAILED_TOKEN_裸跑vite/)
  })

  it('serve 模式 + 错误信息含清晰指引（pm2 / preview-gateway）', () => {
    const plugin = devStartGuard()
    let caught: Error | null = null
    try {
      callGuard(plugin, { command: 'serve' })
    } catch (e) {
      caught = e as Error
    }
    expect(caught).not.toBeNull()
    expect(caught!.message).toContain('pm2 start')
    expect(caught!.message).toContain('preview-gateway')
    expect(caught!.message).toContain('16666')
  })

  // === CI + serve 优先级：CI 在前先放行（不被 PM2 检查影响） ===
  it('serve 模式 + CI=true + 显式 PM2_HOME 缺失 → 不抛错（CI 先于 PM2 判定）', () => {
    process.env.CI = 'true'
    // PM2_HOME 没设
    const plugin = devStartGuard({ errorMessage: 'should not throw' })
    expect(() => callGuard(plugin, { command: 'serve' })).not.toThrow()
  })
})
