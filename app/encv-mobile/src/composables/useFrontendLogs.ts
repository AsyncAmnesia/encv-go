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

function addLog(level: string, args: any[]) {
  logs.value.push({
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level,
    message: args.map((a) => typeof a === 'string' ? a : JSON.stringify(a)).join(' '),
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
    const first = args[0]
    if (typeof first !== 'string') return false
    return (
      first.includes('failed to connect to websocket') ||
      first.includes('WebSocket closed without opened')
    )
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
