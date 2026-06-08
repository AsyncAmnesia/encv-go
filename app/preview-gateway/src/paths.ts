/**
 * paths.ts — 二进制 / 目录路径自动探测 + env 覆盖
 * =====================================================
 *
 * 设计：所有路径都允许 env 覆盖（CI 灵活性），但有合理默认（开箱即用）。
 * 默认值基于本仓库布局（monorepo root = /workspace）。
 *
 * ⚠️ 不要在这里写 vite.config.ts 内容 — 这里只负责"二进制在哪里"。
 */

import { existsSync } from 'node:fs'
import { execSync } from 'node:child_process'

const LOG_PREFIX = '[paths]'

function log(...args: unknown[]): void {
  console.log(LOG_PREFIX, ...args)
}

/** 第一个非空且存在的路径 */
function firstExisting(...candidates: string[]): string | undefined {
  for (const p of candidates) {
    if (p && existsSync(p)) return p
  }
  return undefined
}

/** 用 `which` 找命令（仅在 PATH 中） */
function which(bin: string): string | undefined {
  try {
    const out = execSync(`which ${bin}`, { stdio: ['ignore', 'pipe', 'ignore'] }).toString().trim()
    return out.length > 0 ? out : undefined
  } catch {
    return undefined
  }
}

export interface ResolvedPaths {
  repoRoot: string
  mobileDir: string
  pluginWebDir: string
  airBin: string
  nodeBin: string
  viteJsMain: string
  viteJsPlugin: string
  openlistScript: string
  previewStubJs: string
}

/**
 * 解析所有路径。env 优先级最高，否则自动探测。
 * 任何"必备路径"找不到会 throw — caller 应当在启动早期 fail-fast。
 */
export function resolvePaths(): ResolvedPaths {
  const repoRoot = process.env.REPO_ROOT ?? '/workspace'
  const mobileDir = process.env.MOBILE_DIR ?? `${repoRoot}/app/encv-mobile`
  const pluginWebDir = process.env.PLUGIN_WEB_DIR ?? `${mobileDir}/plugin-openlist/web`

  // air — 优先 env，否则 PATH，否则 mise/go 标准位置
  const airBin =
    process.env.AIR_BIN ??
    which('air') ??
    firstExisting(
      `${process.env.HOME ?? '/root'}/go/bin/air`,
      `${process.env.HOME ?? '/root'}/.local/share/mise/installs/go/1.25.1/bin/air`,
    )
  if (!airBin) {
    throw new Error('air binary not found. Set AIR_BIN env or run setup-sandbox-env.sh')
  }

  // node — 必备
  const nodeBin = process.env.NODE_BIN ?? which('node')
  if (!nodeBin) {
    throw new Error('node binary not found. Set NODE_BIN env or install Node.js')
  }

  // vite.js — 主 app + plugin
  const viteJsMain = process.env.VITE_JS_MAIN ?? `${mobileDir}/node_modules/vite/bin/vite.js`
  const viteJsPlugin = process.env.VITE_JS_PLUGIN ?? `${pluginWebDir}/node_modules/vite/bin/vite.js`
  if (!existsSync(viteJsMain)) {
    throw new Error(`main vite.js not found: ${viteJsMain} (run setup-sandbox-env.sh to pnpm install)`)
  }

  // dev-openlist.sh — 可选，没找到也不 throw（默认 SPAWN_OPENLIST=0）
  const openlistScript = process.env.OPENLIST_SCRIPT ?? `${mobileDir}/scripts/dev-openlist.sh`

  // preview-stub.js — preview-helper 同源（OpenPreview 工具垫脚石）
  const previewStubJs = process.env.PREVIEW_STUB_JS ?? `${repoRoot}/scripts/openpreview-stub.js`

  const resolved: ResolvedPaths = {
    repoRoot,
    mobileDir,
    pluginWebDir,
    airBin,
    nodeBin,
    viteJsMain,
    viteJsPlugin,
    openlistScript,
    previewStubJs,
  }
  log('resolved paths:')
  for (const [k, v] of Object.entries(resolved)) {
    log(`  ${k.padEnd(16)} = ${v}`)
  }
  return resolved
}
