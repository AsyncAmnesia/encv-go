/**
 * Mock 数据生成 / 重置 — 通过后端 API
 *
 * 真机 release 构建下，前端无法直接写磁盘到 /storage/emulated/0/，
 * 所以走后端 SSE 接口：
 *  - POST /api/mock/generate { root, type } → SSE 流式进度
 *  - POST /api/mock/reset { root } → JSON { removed }
 *
 * 安全：后端 white-list 校验 root 前缀（必须是绝对路径，在 /storage/emulated/0[/encv-automation] 等白名单内）。
 * 显式意图：必须带 X-Confirm-Mock-Mutation header（防擅自生成）。
 *
 * 2026-06-10 改造：Node CLI scripts/generate-mock-files.ts 已废弃，本 wrapper 仍是前端调用后端的唯一入口。
 * dev 模式 mock 生成也走后端 API（不带 CLI，避免双源）。
 */
import { getApiBaseUrl } from '@/api/encv'
import type { MockFileType } from '@/lib/mockDataGenerator'

export interface MockProgress {
  relativePath: string
  size: number
}

export interface MockGenerateOptions {
  root: string
  type?: MockFileType
  onProgress?: (p: MockProgress) => void
  /**
   * 🆕 2026-06-11 v4：被跳过的文件（通常是 ffmpeg build 没编该 encoder）
   *   例如：real device 没 libmp3lame/flac → mp3/flac 生成会 emit "skipped" error 事件
   *   区别于 fatal error（fatal 会 throw 中断流，skipped 仅 warning）
   */
  onSkipped?: (info: { relativePath: string; reason: string }) => void
  signal?: AbortSignal
  /**
   * 🆕 2026-06-11 v4：超时（毫秒）。默认 30s。超过后 abort 请求，promise reject → catch 块 → inline error UI
   * 历史 bug：real device 偶发 cgo dlopen 阻塞 → gin SSE 不响应 → 用户看到 spinner 转圈 → 体感"崩溃"
   * ⚠️ 注意：后端 cgo 阻塞无法被 context 取消（CallFFmpegNative 阻塞 cgo call 不响应 ctx.Done）
   *   因此 abort 只会断开 SSE 连接 + 触发 catch 块显示错误，**后端 goroutine 可能继续泄漏**
   *   但用户感知 30s 内能看到 inline error UI，不会再"静默失败"
   * 未来重构：把 ffmpeg 调 subprocess 化彻底解决
   */
  timeoutMs?: number
}

export interface MockGenerateResult {
  count: number
  /** 🆕 2026-06-11 v4：因 ffmpeg build 限制被跳过的文件数（real device 上 mp3/flac 常见） */
  skipped: number
  totalSize: number
}

export interface MockResetResult {
  removed: number
}

/**
 * 通过 SSE 流式拉取生成进度。
 * 后端的事件格式：`data: {"relativePath": "...", "size": 1234}\n\n`
 * 结束时事件：`event: done\ndata: {"count": N, "skipped": K, "totalSize": M}\n\n`
 */
export async function generateMockFilesViaBackend(opts: MockGenerateOptions): Promise<MockGenerateResult> {
  const baseUrl = getApiBaseUrl()
  // 🆕 v4：硬超时（30s），避免后端 hang 时前端 spinner 永远转
  const timeoutMs = opts.timeoutMs ?? 30000
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(new Error('mock generate timeout')), timeoutMs)
  // 兼容外部 signal：若外部 abort 也带过来
  opts.signal?.addEventListener('abort', () => ctrl.abort(opts.signal!.reason))

  let res: Response
  try {
    res = await fetch(`${baseUrl}/api/mock/generate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
        'X-Confirm-Mock-Mutation': 'yes', // 🆕 2026-06-10：显式意图确认（防擅自生成）
      },
      body: JSON.stringify({ root: opts.root, type: opts.type ?? 'all' }),
      signal: ctrl.signal,
    })
  } catch (e) {
    clearTimeout(timer)
    throw e
  }
  if (!res.ok || !res.body) {
    clearTimeout(timer)
    const txt = await res.text().catch(() => '')
    throw new Error(`Mock generate failed (${res.status}): ${txt}`)
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  // 🆕 v4：默认 skipped: 0（兼容旧后端不返回该字段）
  let final: MockGenerateResult = { count: 0, skipped: 0, totalSize: 0 }

  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // 按 SSE 协议分割（\n\n）
      let idx
      while ((idx = buffer.indexOf('\n\n')) >= 0) {
        const eventBlock = buffer.slice(0, idx)
        buffer = buffer.slice(idx + 2)
        const parsed = parseSseEvent(eventBlock)
        if (!parsed) continue
        if (parsed.event === 'progress') {
          try {
            const data = JSON.parse(parsed.data) as MockProgress
            opts.onProgress?.(data)
          } catch {}
        } else if (parsed.event === 'done') {
          try {
            final = JSON.parse(parsed.data) as MockGenerateResult
            // 兼容旧后端（没有 skipped 字段）
            if (typeof final.skipped !== 'number') final.skipped = 0
          } catch {}
        } else if (parsed.event === 'error') {
          // 🆕 v4：区分 skipped（信息）vs fatal error（中断流）
          //   backend emit 格式：{"skipped": true, "relativePath": "...", "reason": "..."} → 走 onSkipped
          //   backend emit 格式：{"error": "..."} → 走 fatal throw
          try {
            const errObj = JSON.parse(parsed.data) as Record<string, unknown>
            if (errObj && errObj.skipped === true) {
              opts.onSkipped?.({
                relativePath: String(errObj.relativePath ?? '?'),
                reason: String(errObj.reason ?? 'unknown'),
              })
              continue
            }
            // fatal error
            throw new Error(parsed.data)
          } catch (e) {
            // JSON 解析失败 → 当 fatal 处理
            if (e instanceof Error && e.message === parsed.data) throw e
            throw new Error(parsed.data)
          }
        }
      }
    }
  } finally {
    clearTimeout(timer)
  }

  return final
}

export async function resetMockFilesViaBackend(root: string): Promise<MockResetResult> {
  const baseUrl = getApiBaseUrl()
  const res = await fetch(`${baseUrl}/api/mock/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ root }),
  })
  if (!res.ok) {
    const txt = await res.text().catch(() => '')
    throw new Error(`Mock reset failed (${res.status}): ${txt}`)
  }
  return res.json()
}

function parseSseEvent(block: string): { event: string; data: string } | null {
  let event = 'message'
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (dataLines.length === 0 && event === 'message') return null
  return { event, data: dataLines.join('\n') }
}
