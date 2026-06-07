import { ref } from 'vue'

export interface LogEntry {
  id: number
  timestamp: string
  level: string
  message: string
}

let nextId = 0
const logs = ref<LogEntry[]>([])

let origConsole: {
  debug: Console['debug']
  info: Console['info']
  warn: Console['warn']
  error: Console['error']
  log: Console['log']
} | null = null

/**
 * 安全序列化任意值为可读字符串。
 * 解决核心痛点：JSON.stringify(new Error('test')) → '{}' （Error 属性不可枚举）
 */
function safeStringify(v: any): string {
  if (v == null) return '(null)'
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  // Error / TypeError / DOMException 等
  if (v instanceof Error) {
    const parts: string[] = [v.name || 'Error', v.message || '(no message)']
    if ((v as any).cause) parts.push(`cause=${safeStringify((v as any).cause)}`)
    return parts.filter(Boolean).join(': ')
  }
  // Event 对象
  if (v instanceof Event) {
    const code = (v as any).code
    return code ? v.type + ' (code=' + code + ')' : v.type
  }
  // Response 对象
  if (v instanceof Response) {
    return `HTTP ${v.status} ${v.statusText}`
  }
  // 普通对象 / 数组
  try {
    const s = JSON.stringify(v)
    return s !== '{}' ? s : Object.prototype.toString.call(v)
  } catch {
    return Object.prototype.toString.call(v)
  }
}

function addLog(level: string, args: any[]) {
  logs.value.push({
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level,
    message: args.map(safeStringify).join(' '),
  })
  if (logs.value.length > 2000) {
    logs.value = logs.value.slice(-1500)
  }
}

export function hijackConsole() {
  if (origConsole) return
  const saved = {
    debug: console.debug,
    info: console.info,
    warn: console.warn,
    error: console.error,
    log: console.log,
  }
  origConsole = saved
  // 沙箱预览（16000 → 2025 → 5173）链路下 agent-tool-host 不支持 WebSocket 升级，
  // vite HMR client 会持续报 `failed to connect to websocket`。
  // 这是预期内的环境噪声，不影响应用运行（vites 仍能 deliver 模块，
  // 只是没有热重载）。在 DevLogs 里把它降级为 debug 级别，避免淹没真正的错误。
  const isHmrWsNoise = (args: any[]): boolean => {
    if (args.length === 0) return false
    // 检查每个参数（字符串或 Error 对象的 message）
    for (const arg of args) {
      const s = typeof arg === 'string' ? arg : (arg?.message ?? '')
      if (s.includes('failed to connect to websocket') || s.includes('WebSocket closed without opened')) {
        return true
      }
    }
    return false
  }
  console.debug = (...args: any[]) => { saved.debug(...args); addLog('debug', args) }
  console.info = (...args: any[]) => { saved.info(...args); addLog('info', args) }
  console.warn = (...args: any[]) => { saved.warn(...args); addLog('warn', args) }
  console.error = (...args: any[]) => {
    saved.error(...args)
    if (isHmrWsNoise(args)) {
      addLog('debug', ['[HMR WS sandbox noise] ' + args[0]])
      return
    }
    addLog('error', args)
  }
  console.log = (...args: any[]) => { saved.log(...args); addLog('info', args) }
}

export function restoreConsole() {
  if (!origConsole) return
  console.debug = origConsole.debug
  console.info = origConsole.info
  console.warn = origConsole.warn
  console.error = origConsole.error
  console.log = origConsole.log
  origConsole = null
}

export function clearFrontendLogs() {
  logs.value = []
}

export function useFrontendLogs() {
  return {
    logs,
    clearLogs: clearFrontendLogs,
  }
}

export function getFrontendLogsJson(): string {
  return JSON.stringify(logs.value, null, 2)
}
