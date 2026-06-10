/**
 * preflight.ts — gateway 启动前的预检
 * =====================================================
 *
 * 唯一职责：在子进程启动前确保 mock 数据存在 / 关键目录就绪。
 * 这块从 start-preview.sh 抽出来，是 mobile overlay 触发的必要前提。
 *
 * 关键不变量：
 *   - /storage/emulated/0 必须存在且含 01-plain-media
 *   - encv-go 启动后会读 ENCV_MOCK_ROOT 决定 servingDir
 *   - 没有 mock 数据 → mobile overlay 看到空目录 → 用户困惑
 */

import { spawn } from 'node:child_process'
import { existsSync, mkdirSync } from 'node:fs'

const LOG_PREFIX = '[preflight]'

function log(...args: unknown[]): void {
  console.log(LOG_PREFIX, ...args)
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** 同步跑子进程 + 等完成，**透传 stderr**（防止 tsx 错误被静默吞掉） */
function runSync(cmd: string, args: string[], cwd: string, env: NodeJS.ProcessEnv): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { cwd, env, stdio: ['ignore', 'inherit', 'pipe'] })
    let stderr = ''
    // 透传 stderr（stdio=inherit 已自动，但保留 collector 抓取最后 2KB 用于错误信息）
    const stderrStream = child.stderr as NodeJS.ReadableStream | null
    if (stderrStream) {
      stderrStream.on('data', (chunk: Buffer) => { stderr += chunk.toString() })
    }
    child.on('exit', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`${cmd} exited with code ${code}\n--- stderr ---\n${stderr.slice(-2000)}`))
    })
    child.on('error', (err) => reject(new Error(`spawn ${cmd} failed: ${err.message}`)))
  })
}

/**
 * 确保 mock 数据存在。失败抛错（让 gateway 退出 → pm2 重启）。
 *
 * 步骤：
 *   1. mkdir -p ${ENCV_MOCK_ROOT:-/storage/emulated/0}
 *   2. 若 ${ENCV_MOCK_ROOT}/01-plain-media 不存在 → 跑 npx tsx scripts/generate-mock-files.ts
 */
export async function ensureMockData(mobileDir: string): Promise<void> {
  const mockRoot = process.env.ENCV_MOCK_ROOT ?? '/storage/emulated/0'
  log(`mock root: ${mockRoot}`)

  // 1. 确保目录存在
  if (!existsSync(mockRoot)) {
    log(`creating ${mockRoot}`)
    try {
      mkdirSync(mockRoot, { recursive: true })
    } catch (err) {
      throw new Error(`cannot create ${mockRoot}: ${(err as Error).message}`)
    }
  }

  // 2. 检测 01-plain-media 标记目录
  const marker = `${mockRoot}/01-plain-media`
  if (existsSync(marker)) {
    log(`mock data present (${marker}), skip generation`)
    return
  }

  // 3. 跑 mock 生成
  log(`generating mock data via tsx scripts/generate-mock-files.ts ...`)
  await runSync('npx', ['tsx', 'scripts/generate-mock-files.ts'], mobileDir, process.env)
  // 给文件系统一点时间
  await sleep(200)
  if (!existsSync(marker)) {
    throw new Error(`mock generation claimed success but ${marker} still missing`)
  }
  log(`mock data generated ✓`)
}
