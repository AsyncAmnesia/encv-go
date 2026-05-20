const SERVER_URL_KEY = 'encv-server-url'
export const DEFAULT_API_BASE_URL = 'http://127.0.0.1:2025'

function getApiBaseUrl(): string {
  if (import.meta.env.DEV) return ''
  return localStorage.getItem(SERVER_URL_KEY) || DEFAULT_API_BASE_URL
}

export function setApiBaseUrl(url: string) {
  localStorage.setItem(SERVER_URL_KEY, url)
}

export function getServerUrl(): string {
  return getApiBaseUrl()
}

export function resetServerUrl() {
  localStorage.removeItem(SERVER_URL_KEY)
}

export function getWebSocketUrl(): string {
  if (import.meta.env.DEV) {
    const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${wsProtocol}//${location.host}/ws`
  }
  const baseUrl = getApiBaseUrl()
  const wsUrl = baseUrl
    .replace(/^https:\/\//, 'wss://')
    .replace(/^http:\/\//, 'ws://')
  return `${wsUrl}/ws`
}

export interface FileItem {
  name: string
  path: string
  isDirectory: boolean
  size?: number
  modified?: string
}

export interface FileListResponse {
  files: FileItem[]
}

export async function listFiles(path = '/'): Promise<FileItem[]> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files?path=${encodeURIComponent(path)}`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data: FileListResponse = await response.json()
  return data.files || []
}

export function getFileStreamUrl(path: string): string {
  if (import.meta.env.DEV) {
    return `/stream?path=${encodeURIComponent(path)}`
  }
  const baseUrl = getApiBaseUrl()
  return `${baseUrl}/stream?path=${encodeURIComponent(path)}`
}

export async function checkServerStatus(): Promise<{ online: boolean; error?: string }> {
  try {
    const baseUrl = getApiBaseUrl()
    const response = await fetch(`${baseUrl}/health`)
    if (response.ok) {
      return { online: true }
    }
    return { online: false, error: `HTTP ${response.status}` }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    return { online: false, error: msg }
  }
}

export async function deleteFile(path: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export interface FileContentResponse {
  name: string
  path: string
  size: number
  content: string
  encoding: string
}

export async function readFileContent(path: string): Promise<FileContentResponse> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/file?path=${encodeURIComponent(path)}`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export type TaskType = 'encrypt' | 'decrypt'
export type TaskStatus = 'queued' | 'running' | 'completed' | 'failed'

export interface EncvTask {
  id: string
  type: TaskType
  sourcePath: string
  status: TaskStatus
  progress: number
  error?: string
  createdAt: string
}

export async function getTasks(): Promise<EncvTask[]> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data = await response.json()
  return data.tasks || []
}

export async function createTask(type: TaskType, sourcePath: string): Promise<EncvTask> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ type, sourcePath }),
  })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export async function cancelTask(id: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks/${id}/cancel`, {
    method: 'POST',
  })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function retryTask(id: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks/${id}/retry`, {
    method: 'POST',
  })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export interface WebDAVConfig {
  id: string
  name: string
  url: string
  username: string
  password: string
  mountPath: string
}

const WEBDAV_CONFIGS_KEY = 'encv-webdav-configs'

export function getWebDAVConfigs(): WebDAVConfig[] {
  const stored = localStorage.getItem(WEBDAV_CONFIGS_KEY)
  return stored ? JSON.parse(stored) : []
}

export function saveWebDAVConfigs(configs: WebDAVConfig[]) {
  localStorage.setItem(WEBDAV_CONFIGS_KEY, JSON.stringify(configs))
}

export async function testWebDAVConnection(config: Omit<WebDAVConfig, 'id'>): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/webdav/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
  if (!response.ok) {
    let detail = `HTTP ${response.status}`
    try {
      const body = await response.text()
      if (body) detail += `: ${body}`
    } catch {}
    throw new Error(detail)
  }
}

export function formatFileSize(bytes?: number): string {
  if (bytes === undefined || bytes === null) return ''
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${units[i]}`
}

export function getFileExtension(name: string): string {
  const lastDot = name.lastIndexOf('.')
  if (lastDot === -1) return ''
  return name.substring(lastDot + 1).toLowerCase()
}

export type FileCategory = 'video' | 'audio' | 'image' | 'document' | 'encrypted' | 'other'

export function getFileCategory(name: string): FileCategory {
  const ext = getFileExtension(name)
  const videoExts = ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'webm', 'm4v']
  const audioExts = ['mp3', 'flac', 'wav', 'aac', 'ogg', 'wma', 'm4a']
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg']
  const docExts = ['pdf', 'doc', 'docx', 'txt', 'xls', 'xlsx', 'ppt', 'pptx']

  if (ext === 'encv') return 'encrypted'
  if (videoExts.includes(ext)) return 'video'
  if (audioExts.includes(ext)) return 'audio'
  if (imageExts.includes(ext)) return 'image'
  if (docExts.includes(ext)) return 'document'
  return 'other'
}

export async function fetchConfig(): Promise<Record<string, unknown>> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/config`)
  if (!response.ok) {
    let detail = `HTTP ${response.status}`
    try {
      const body = await response.text()
      if (body) detail += `: ${body}`
    } catch {}
    throw new Error(detail)
  }
  return await response.json()
}

export async function updateConfig(config: Record<string, unknown>): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
  if (!response.ok) {
    let detail = `HTTP ${response.status}`
    try {
      const body = await response.text()
      if (body) detail += `: ${body}`
    } catch {}
    throw new Error(detail)
  }
}

export async function fetchConfigSchema(): Promise<Record<string, unknown>> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/config/schema`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}
