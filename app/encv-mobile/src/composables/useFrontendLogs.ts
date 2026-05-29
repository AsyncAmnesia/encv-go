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
  console.debug = (...args: any[]) => { saved.debug(...args); addLog('debug', args) }
  console.info = (...args: any[]) => { saved.info(...args); addLog('info', args) }
  console.warn = (...args: any[]) => { saved.warn(...args); addLog('warn', args) }
  console.error = (...args: any[]) => { saved.error(...args); addLog('error', args) }
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
