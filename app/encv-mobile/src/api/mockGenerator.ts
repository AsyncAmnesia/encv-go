/**
 * Mock 数据生成 / 重置 — 通过后端 API
 *
 * 真机 release 构建下，前端无法直接写磁盘到 /storage/emulated/0/，
 * 所以走后端 SSE 接口：
 *  - POST /api/mock/generate { root, type } → SSE 流式进度
 *  - POST /api/mock/reset { root } → JSON { removed }
 *
 * dev 模式下也可以调（dev 模式 dev 选项传入的 MOCK_ROOT 也走这个接口更稳）。
 *
 * 安全：后端 white-list 校验 root 前缀（dev: __mock_data__/，真机: encv-automation/）。
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
  signal?: AbortSignal
}

export interface MockGenerateResult {
  count: number
  totalSize: number
}

export interface MockResetResult {
  removed: number
}

/**
 * 通过 SSE 流式拉取生成进度。
 * 后端的事件格式：`data: {"relativePath": "...", "size": 1234}\n\n`
 * 结束时事件：`event: done\ndata: {"count": N, "totalSize": M}\n\n`
 */
export async function generateMockFilesViaBackend(opts: MockGenerateOptions): Promise<MockGenerateResult> {
  const baseUrl = getApiBaseUrl()
  const res = await fetch(`${baseUrl}/api/mock/generate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'text/event-stream',
    },
    body: JSON.stringify({ root: opts.root, type: opts.type ?? 'all' }),
    signal: opts.signal,
  })
  if (!res.ok || !res.body) {
    const txt = await res.text().catch(() => '')
    throw new Error(`Mock generate failed (${res.status}): ${txt}`)
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let final: MockGenerateResult = { count: 0, totalSize: 0 }

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
        } catch {}
      } else if (parsed.event === 'error') {
        throw new Error(parsed.data)
      }
    }
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
