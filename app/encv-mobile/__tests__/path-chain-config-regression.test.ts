/**
 * path-chain-config-regression.test.ts
 *
 * ⚠️ 关键回归测试：跨链路 ENCV_MOCK_ROOT 一致性
 *
 * 链路：
 *   A. ecosystem.config.cjs 注入的 env（pm2 启动时给 mock 脚本用）
 *   B. scripts/generate-mock-files.ts 的 fallback 常量
 *   C. encv-mobile src 内 DEFAULT_AUTOMATION_SOURCE 的父目录
 *
 * 如果任一处漂移 → Mock 写盘路径 ≠ 任务读盘路径 → "source file not found" 错误
 * （用户报告的问题：2026-06-10 之前 B/C 不一致，B=/storage/emulated/0，
 *  C=/storage/emulated/0/encv-automation）
 *
 * 文件位置说明：此文件在 /workspace/app/encv-mobile/__tests__/（仓库根级），
 * 不在 src/ 里 — 故 tsconfig.json 的 include 范围（src 之下的 ts）不会扫它，
 * vue-tsc --noEmit 不会因 `node:fs` / `node:path` / `node:url` 报 TS2307
 * （frontend tsconfig 不加载 @types/node）。
 *
 * vitest.config.ts 的 include 第 1 条 `__tests__` 下的 test.ts 仍会扫它，
 * 跑测试时 Node 环境原生支持 node:fs/path/url 协议。
 */

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

// __tests__/ → 仓库根
//  - __tests__/ 在 /workspace/app/encv-mobile/__tests__/
//  - 仓库根 = /workspace
//  - __dirname 是 /workspace/app/encv-mobile/__tests__/，上 3 级 = /workspace
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const REPO_ROOT = resolve(__dirname, '..', '..', '..')

// ⚠️ 硬约束：修改前必须先看 usePathResolver.withSafetyBoundary + mobile overlay
//   这是 mobile 端 mock 路径链路 A：servingDir/server.dir → task sourcePath 的唯一根
const EXPECTED_MOCK_ROOT = '/storage/emulated/0/encv-automation'

describe('path-chain — 配置文件防回归（跨链路一致）', () => {
  it('【防回归】ecosystem.config.cjs ENCV_MOCK_ROOT 必须 = /storage/emulated/0/encv-automation', () => {
    const cfg = readFileSync(resolve(REPO_ROOT, 'ecosystem.config.cjs'), 'utf-8')
    // 提取第一个 ENCV_MOCK_ROOT: '...' 值
    const m = cfg.match(/ENCV_MOCK_ROOT:\s*['"]([^'"]+)['"]/)
    expect(m, 'ENCV_MOCK_ROOT must be present in ecosystem.config.cjs').toBeTruthy()
    expect(m![1]).toBe(EXPECTED_MOCK_ROOT)
  })

  it('【防回归】generate-mock-files.ts 的 fallback 必须 = /storage/emulated/0/encv-automation', () => {
    const src = readFileSync(
      resolve(REPO_ROOT, 'app/encv-mobile/scripts/generate-mock-files.ts'),
      'utf-8',
    )
    // 提取 MOCK_ROOT = process.env.ENCV_MOCK_ROOT || '...' 字符串
    const m = src.match(/MOCK_ROOT\s*=\s*process\.env\.ENCV_MOCK_ROOT\s*\|\|\s*['"]([^'"]+)['"]/)
    expect(m, 'MOCK_ROOT fallback must be present in generate-mock-files.ts').toBeTruthy()
    expect(m![1]).toBe(EXPECTED_MOCK_ROOT)
  })

  it('【防回归】A/B 两处 ENCV_MOCK_ROOT 必须完全一致', () => {
    const cfg = readFileSync(resolve(REPO_ROOT, 'ecosystem.config.cjs'), 'utf-8')
    const script = readFileSync(
      resolve(REPO_ROOT, 'app/encv-mobile/scripts/generate-mock-files.ts'),
      'utf-8',
    )

    const cfgMatch = cfg.match(/ENCV_MOCK_ROOT:\s*['"]([^'"]+)['"]/)
    const scriptMatch = script.match(/MOCK_ROOT\s*=\s*process\.env\.ENCV_MOCK_ROOT\s*\|\|\s*['"]([^'"]+)['"]/)

    expect(cfgMatch).toBeTruthy()
    expect(scriptMatch).toBeTruthy()
    expect(cfgMatch![1]).toBe(scriptMatch![1])
    // 锁定到硬约束
    expect(cfgMatch![1]).toBe(EXPECTED_MOCK_ROOT)
    expect(scriptMatch![1]).toBe(EXPECTED_MOCK_ROOT)
  })
})
