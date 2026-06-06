/**
 * agent-stub.js
 * =============
 *
 * 临时 stub，模拟 spec §go-in-process-agent 的 in-process AI agent
 * 直到真实 Go agent 就绪（:5245）。
 *
 * 端点（与 useAgent.ts L483/552/609 对齐）：
 *   POST /api/chat    — 发起对话（SSE）
 *   POST /api/confirm — 4-决策确认（SSE）
 *   POST /api/resume  — 断点续传（SSE）
 *   GET  /test        — 测试连接（{openai: 'ok', openlist: 'ok', model: '...'}）
 *   GET  /__agent/health — 健康检查
 *
 * SSE 事件格式（与 useAgent.ts processSSE 对齐）：
 *   data: {"type": "text_delta", "data": "..."}\n\n
 *   data: {"type": "stream_end",  "data": ""}\n\n
 *
 * 启动：PORT=5245 node scripts/agent-stub.js
 * 由 ecosystem.config.cjs 的 agent-stub 进程监管。
 */
const http = require('node:http')
const { randomUUID } = require('node:crypto')

const PORT = Number(process.env.PORT || 5245)
const HOST = process.env.HOST || '0.0.0.0'

function log(...args) {
  console.log('[agent-stub]', ...args)
}

function readJsonBody(req) {
  return new Promise((resolve, reject) => {
    let raw = ''
    req.on('data', (chunk) => { raw += chunk })
    req.on('end', () => {
      if (!raw) return resolve({})
      try { resolve(JSON.parse(raw)) } catch (e) { reject(e) }
    })
    req.on('error', reject)
  })
}

function setSseHeaders(res) {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream; charset=utf-8',
    'Cache-Control': 'no-cache, no-transform',
    'Connection': 'keep-alive',
    'X-Accel-Buffering': 'no',
  })
  res.write(': agent-stub ok\n\n')
}

function sendEvent(res, type, data) {
  res.write(`data: ${JSON.stringify({ type, data })}\n\n`)
}

/**
 * 把字符串切成 ~chunk 大小的字符流，每隔 ~delayMs 发送一次
 */
function streamText(res, text, { chunkSize = 4, delayMs = 50 } = {}) {
  return new Promise((resolve) => {
    let i = 0
    function tick() {
      if (i >= text.length) {
        resolve()
        return
      }
      const slice = text.slice(i, i + chunkSize)
      i += chunkSize
      sendEvent(res, 'text_delta', slice)
      setTimeout(tick, delayMs)
    }
    tick()
  })
}

/**
 * 合成一段 mock 响应：echo 用户最后一条 + 当前模型信息 + 工具演示
 */
function synthesizeReply(body) {
  const model = body?.model || 'unknown-model'
  const temperature = typeof body?.temperature === 'number' ? body.temperature : 0.7
  const lastUser = (body?.messages || []).filter((m) => m.role === 'user').pop()
  const userText = lastUser?.content || ''
  const lines = [
    `（这是 agent-stub 模拟回复，真实 AI agent 尚未就绪）`,
    ``,
    `📌 收到你的输入：${userText.slice(0, 200)}`,
    ``,
    `⚙️ 当前模型：${model}`,
    `🌡️ 温度：${temperature}`,
    ``,
    `当真实 Go agent 服务就绪后，preview-gateway 的 /agent-api/* 路由会把请求转发到 :5245 并流式返回结果。`,
    ``,
    `你可以：`,
    `1. 在右上角切换模型`,
    `2. 调整温度参数`,
    `3. 等待真实后端集成（见 spec/go-in-process-agent/）`,
  ]
  return lines.join('\n')
}

async function handleChat(req, res) {
  let body
  try { body = await readJsonBody(req) } catch (e) {
    res.writeHead(400, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'invalid_json', detail: String(e) }))
    return
  }
  const sessionId = body?.sessionId || randomUUID()
  log(`chat session=${sessionId} model=${body?.model} temp=${body?.temperature}`)
  setSseHeaders(res)
  sendEvent(res, 'session_start', sessionId)
  const text = synthesizeReply(body)
  await streamText(res, text, { chunkSize: 3, delayMs: 40 })
  sendEvent(res, 'stream_end', '')
  res.end()
  log(`chat session=${sessionId} done (${text.length} chars streamed)`)
}

async function handleConfirm(req, res) {
  let body
  try { body = await readJsonBody(req) } catch (e) {
    res.writeHead(400, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'invalid_json' }))
    return
  }
  log(`confirm tool=${body?.toolCallId} decision=${body?.decision}`)
  setSseHeaders(res)
  const text = `（stub）已收到决策：${body?.decision || 'unknown'}（工具 ${body?.toolCallId || '?'}）`
  await streamText(res, text, { chunkSize: 3, delayMs: 40 })
  sendEvent(res, 'stream_end', '')
  res.end()
}

async function handleResume(req, res) {
  let body
  try { body = await readJsonBody(req) } catch (e) {
    res.writeHead(400, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'invalid_json' }))
    return
  }
  log(`resume session=${body?.sessionId}`)
  setSseHeaders(res)
  const text = `（stub）已恢复 session ${body?.sessionId || '?'}。真实 agent 将从这里继续流式输出。`
  await streamText(res, text, { chunkSize: 3, delayMs: 40 })
  sendEvent(res, 'stream_end', '')
  res.end()
}

async function handleTest(req, res) {
  res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify({
    openai: 'ok',
    openlist: 'ok',
    model: 'gpt-4o-mini (stub)',
    note: 'agent-stub mock response — real agent pending',
  }))
}

async function handleModels(req, res) {
  res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' })
  // 返回模拟模型列表（真实 agent 应从 OpenAI /providers 动态查询）
  const models = [
    { id: 'gpt-4o-mini', name: 'GPT-4o Mini', provider: 'openai' },
    { id: 'gpt-4o', name: 'GPT-4o', provider: 'openai' },
    { id: 'gpt-4.1', name: 'GPT-4.1', provider: 'openai' },
    { id: 'gpt-4.1-mini', name: 'GPT-4.1 Mini', provider: 'openai' },
    { id: 'gpt-4.1-nano', name: 'GPT-4.1 Nano', provider: 'openai' },
    { id: 'o3', name: 'O3', provider: 'openai' },
    { id: 'o3-mini', name: 'O3 Mini', provider: 'openai' },
    { id: 'claude-sonnet-4-20250514', name: 'Claude Sonnet 4', provider: 'anthropic' },
    { id: 'claude-haiku-4-20250514', name: 'Claude Haiku 4', provider: 'anthropic' },
    { id: 'deepseek-chat', name: 'DeepSeek Chat', provider: 'deepseek' },
    { id: 'deepseek-reasoner', name: 'DeepSeek Reasoner', provider: 'deepseek' },
    { id: 'qwen3-coder-plus', name: 'Qwen3 Coder Plus', provider: 'qwen' },
  ]
  res.end(JSON.stringify({ models, defaultModel: 'gpt-4o-mini' }))
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/__agent/health') {
    res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' })
    res.end(JSON.stringify({ ok: true, name: 'agent-stub', port: PORT, ts: Date.now() }))
    return
  }
  if (req.method === 'POST' && req.url === '/api/chat') return handleChat(req, res)
  if (req.method === 'POST' && req.url === '/api/confirm') return handleConfirm(req, res)
  if (req.method === 'POST' && req.url === '/api/resume') return handleResume(req, res)
  if ((req.method === 'GET' || req.method === 'POST') && req.url === '/test') return handleTest(req, res)
  if (req.method === 'GET' && req.url === '/api/models') return handleModels(req, res)
  res.writeHead(404, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify({ error: 'not_found', method: req.method, path: req.url }))
})

server.listen(PORT, HOST, () => {
  log(`listening on http://${HOST}:${PORT} (stub for spec §go-in-process-agent)`)
  log(`routes: POST /api/chat /api/confirm /api/resume  |  GET /__agent/health`)
})

for (const sig of ['SIGINT', 'SIGTERM']) {
  process.on(sig, () => {
    log(`received ${sig}, closing...`)
    server.close(() => process.exit(0))
    setTimeout(() => process.exit(1), 5_000).unref()
  })
}
