const SERVER_URL_KEY = 'encv-server-url'
export const DEFAULT_API_BASE_URL = 'http://127.0.0.1:2025'

export function getApiBaseUrl(): string {
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
  isEncrypted?: boolean
  size?: number
  modified?: string
}

export interface FileListResponse {
  files: FileItem[]
  error?: string
  code?: string
}

export class PermissionDeniedError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'PermissionDeniedError'
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'NotFoundError'
  }
}

export async function listFiles(path = '/'): Promise<FileItem[]> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files?path=${encodeURIComponent(path)}`)
  if (!response.ok) {
    if (response.status === 403) {
      const data: FileListResponse = await response.json().catch(() => ({}))
      if (data.code === 'PERMISSION_DENIED') {
        console.debug('[API] listFiles permission denied:', path)
        throw new PermissionDeniedError(data.error || 'Permission denied')
      }
    }
    if (response.status === 404) {
      const data: FileListResponse = await response.json().catch(() => ({}))
      console.debug('[API] listFiles not found:', path)
      throw new NotFoundError(data.error || 'Path not found')
    }
    console.error('[API] listFiles failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data: FileListResponse = await response.json()
  console.info('[API] listFiles:', path, '→', data.files?.length || 0, 'files')
  return data.files || []
}

export async function listFilesStream(
  path = '/',
  onItem: (file: FileItem) => void,
  signal?: AbortSignal
): Promise<{ files: FileItem[]; error?: string }> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files/stream?path=${encodeURIComponent(path)}`, {
    signal,
  })

  if (!response.ok) {
    if (response.status === 403) {
      throw new PermissionDeniedError('Permission denied')
    }
    if (response.status === 404) {
      throw new NotFoundError('Path not found')
    }
    throw new Error(`HTTP error! status: ${response.status}`)
  }

  const files: FileItem[] = []
  const reader = response.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data) continue

        if (data === '[DONE]') {
          return { files }
        }

        try {
          const file = JSON.parse(data) as FileItem
          files.push(file)
          onItem(file)
        } catch {
          // skip malformed JSON
        }
      }
    }
  } finally {
    reader.releaseLock()
  }

  return { files }
}

export async function listPluginFilesStream(
  path: string,
  extensions: string[],
  onItem: (file: FileItem) => void,
  signal?: AbortSignal
): Promise<{ files: FileItem[]; error?: string }> {
  const baseUrl = getApiBaseUrl()
  const extParam = extensions.map(e => `.${e.toLowerCase()}`).join(',')
  const response = await fetch(`${baseUrl}/api/files/plugin-stream?path=${encodeURIComponent(path)}&extensions=${encodeURIComponent(extParam)}`, {
    signal,
  })

  if (!response.ok) {
    if (response.status === 403) {
      throw new PermissionDeniedError('Permission denied')
    }
    throw new Error(`HTTP error! status: ${response.status}`)
  }

  const files: FileItem[] = []
  const reader = response.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data) continue

        if (data === '[DONE]') {
          return { files }
        }

        try {
          const file = JSON.parse(data) as FileItem
          files.push(file)
          onItem(file)
        } catch {
          // skip malformed JSON
        }
      }
    }
  } finally {
    reader.releaseLock()
  }

  return { files }
}

export interface BackendPermissions {
  storage: boolean
}

export async function checkBackendPermissions(): Promise<BackendPermissions> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/permissions`)
  if (!response.ok) {
    console.debug('[API] checkPermissions failed:', response.status)
    return { storage: false }
  }
  const result = await response.json()
  console.info('[API] permissions:', JSON.stringify(result))
  return result
}

export function getFileStreamUrl(path: string): string {
  if (import.meta.env.DEV) {
    return `/stream?path=${encodeURIComponent(path)}`
  }
  const baseUrl = getApiBaseUrl()
  return `${baseUrl}/stream?path=${encodeURIComponent(path)}`
}

export function getFilePreviewUrl(previewPage: string, filePath: string): string {
  if (import.meta.env.DEV) {
    return `/preview/${previewPage}?file=${encodeURIComponent(filePath)}`
  }
  const baseUrl = getApiBaseUrl()
  return `${baseUrl}/preview/${previewPage}?file=${encodeURIComponent(filePath)}`
}

export function getExternalStreamUrl(path: string): string {
  if (import.meta.env.DEV) {
    return `/api/stream/external?path=${encodeURIComponent(path)}`
  }
  const baseUrl = getApiBaseUrl()
  return `${baseUrl}/api/stream/external?path=${encodeURIComponent(path)}`
}

export async function checkServerStatus(): Promise<{ online: boolean; error?: string }> {
  try {
    const baseUrl = getApiBaseUrl()
    const response = await fetch(`${baseUrl}/health`)
    if (response.ok) {
      console.info('[API] server online')
      return { online: true }
    }
    return { online: false, error: `HTTP ${response.status}` }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    console.debug('[API] server offline:', msg)
    return { online: false, error: msg }
  }
}

export async function deleteFile(path: string): Promise<void> {
  console.debug('[API] deleteFile:', path)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    console.error('[API] deleteFile failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function createDirectory(parentPath: string, name: string): Promise<void> {
  console.info('[API] createDirectory:', parentPath, name)
  const response = await fetch(`${getApiBaseUrl()}/api/files/mkdir`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parent_path: parentPath, name }),
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || `Failed to create directory (${response.status})`)
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
    console.error('[API] readFileContent failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data = await response.json()
  console.info('[API] readFileContent:', path, 'size:', data.size)
  return data
}

export type TaskType = 'encrypt' | 'decrypt'
export type TaskStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'cancelling'

export interface EncvTask {
  id: string
  type: TaskType
  sourcePath: string
  targetPath?: string
  pluginName?: string
  status: TaskStatus
  progress: number
  phase?: string
  speed?: string
  eta?: string
  error?: string
  errorDetail?: string
  warning?: string
  warningDetail?: string
  containerVersion?: number
  createdAt: string
  completedAt?: string
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

export async function createTask(
  type: TaskType,
  sourcePath: string,
  targetPath?: string,
  password?: string,
  version?: number,
  pluginName?: string,
  extraFields?: Record<string, string>,
  secondaryPassword?: string,
): Promise<EncvTask> {
  console.info('[API] createTask:', type, sourcePath, targetPath || '',
    'hasPassword:', !!password, 'version:', version ?? 'default',
    'pluginName:', pluginName ?? 'auto',
    'hasExtraFields:', extraFields && Object.keys(extraFields).length > 0,
    'hasSecondaryPassword:', !!secondaryPassword)
  const baseUrl = getApiBaseUrl()
  const body: Record<string, unknown> = { type, sourcePath }
  if (targetPath) body.targetPath = targetPath
  if (password) body.password = password
  if (version) body.version = version
  if (pluginName) body.pluginName = pluginName
  if (extraFields && Object.keys(extraFields).length > 0) body.extraFields = extraFields
  if (secondaryPassword) body.secondaryPassword = secondaryPassword
  const response = await fetch(`${baseUrl}/api/tasks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
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

export async function removeTask(id: string): Promise<void> {
  console.info('[API] removeTask:', id)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks/${id}`, {
    method: 'DELETE',
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
  isBuiltIn?: boolean
}

export interface RemoteWebDAVInfo {
  enabled: boolean
  url: string
  username: string
  root: string
}

export interface OpenlistSiteInfo {
  host: string
  description: string
  proxyUrl: string
}

export interface RemoteInfo {
  webdav: RemoteWebDAVInfo
  openlistSites: Record<string, OpenlistSiteInfo>
}

const WEBDAV_CONFIGS_KEY = 'encv-webdav-configs'

export function getWebDAVConfigs(): WebDAVConfig[] {
  const stored = localStorage.getItem(WEBDAV_CONFIGS_KEY)
  return stored ? JSON.parse(stored) : []
}

export function saveWebDAVConfigs(configs: WebDAVConfig[]) {
  localStorage.setItem(WEBDAV_CONFIGS_KEY, JSON.stringify(configs))
}

export interface LocalWebDAVTestResult {
  available: boolean
  url?: string
  authRequired?: boolean
  details?: {
    propfindRoot: string
    authWorks: string
    dirReadable: string
  }
  error?: string
}

export async function testLocalWebDAV(): Promise<LocalWebDAVTestResult> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/webdav/test-local`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export interface WebDAVTestResult {
  success: boolean
  reachable: boolean
  is_webdav: boolean
  auth_ok: boolean
  dir_readable: boolean
  status_code: number
  dav_header?: string
  error?: string
}

export async function testWebDAVConnection(config: Omit<WebDAVConfig, 'id'>): Promise<WebDAVTestResult> {
  console.info('[API] testWebDAV')
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
  const data = await response.json()
  if (data.success === false) {
    const result = data as WebDAVTestResult
    return result
  }
  return data as WebDAVTestResult
}

export function formatFileSize(bytes?: number): string {
  if (bytes === undefined || bytes === null) return ''
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${units[i]}`
}

export interface TextPreviewExts {
  extensions: string[]
  custom_extensions: string[]
}

let cachedTextExts: Set<string> | null = null

export async function fetchTextPreviewExts(): Promise<Set<string>> {
  if (cachedTextExts) return cachedTextExts
  const baseUrl = getApiBaseUrl()
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 5000)
  try {
    const response = await fetch(`${baseUrl}/api/file/text-preview-exts`, { signal: controller.signal })
    if (!response.ok) {
      console.error('[API] fetchTextPreviewExts failed:', response.status)
      return new Set()
    }
    const data = await response.json() as TextPreviewExts
    const all = new Set([...(data.extensions || []), ...(data.custom_extensions || [])])
    cachedTextExts = all
    return all
  } catch (err: any) {
    if (err?.name === 'AbortError') {
      console.debug('[API] fetchTextPreviewExts timed out after 5s')
    } else {
      console.error('[API] fetchTextPreviewExts error:', err)
    }
    return new Set()
  } finally {
    clearTimeout(timer)
  }
}

export function isTextPreviewable(name: string): boolean {
  if (!cachedTextExts) return false
  const ext = getFileExtension(name)
  return cachedTextExts.has(ext)
}

export function invalidateTextExtsCache(): void {
  cachedTextExts = null
}

export function getFileExtension(name: string): string {
  const lastDot = name.lastIndexOf('.')
  if (lastDot === -1) return ''
  return name.substring(lastDot + 1).toLowerCase()
}

export type FileCategory = 'video' | 'audio' | 'image' | 'document' | 'other'

export function getFileCategory(name: string): FileCategory {
  const ext = getFileExtension(name)
  const videoExts = ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'webm', 'm4v']
  const audioExts = ['mp3', 'flac', 'wav', 'aac', 'ogg', 'wma', 'm4a']
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg']
  const docExts = ['pdf', 'doc', 'docx', 'txt', 'xls', 'xlsx', 'ppt', 'pptx']

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

export async function updateConfig(config: Record<string, unknown>): Promise<{ message: string; needsRestart?: boolean }> {
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
  try {
    return await response.json()
  } catch {
    return { message: 'config updated' }
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

export async function searchFiles(path: string, keyword: string, recursive = false): Promise<FileItem[]> {
  const baseUrl = getApiBaseUrl()
  const params = new URLSearchParams({
    path,
    keyword,
    recursive: String(recursive),
  })
  const response = await fetch(`${baseUrl}/api/files/search?${params}`)
  if (!response.ok) {
    if (response.status === 403) {
      const data = await response.json().catch(() => ({}))
      if (data.code === 'PERMISSION_DENIED') {
        throw new PermissionDeniedError(data.error || 'Permission denied')
      }
    }
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data = await response.json()
  return data.files || []
}

export interface IndexStats {
  totalFiles: number
  totalDirs: number
  totalSize: number
  indexedAt: string
  isIndexing: boolean
  lastBuildMs: number
  source?: string
  containers?: number
}

export async function getIndexStats(): Promise<IndexStats> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/index/stats`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export async function rebuildIndex(): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/index/rebuild`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function clearIndex(): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/index/clear`, { method: 'POST' })
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function fetchRemoteInfo(): Promise<RemoteInfo> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/remote/info`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export async function addOpenlistSite(siteId: string, host: string, description: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/remote/openlist`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ siteId, host, description }),
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || `HTTP ${response.status}`)
  }
}

export async function updateOpenlistSite(siteId: string, host: string, description: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/remote/openlist/${siteId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ host, description }),
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || `HTTP ${response.status}`)
  }
}

export async function deleteOpenlistSite(siteId: string): Promise<void> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/remote/openlist/${siteId}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || `HTTP ${response.status}`)
  }
}

export async function checkFileExists(path: string): Promise<boolean> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files/exists?path=${encodeURIComponent(path)}`)
  if (!response.ok) {
    console.debug('[API] checkFileExists failed:', response.status)
    return false
  }
  const data = await response.json()
  return !!data.exists
}

export async function checkEncryptOutputExists(sourcePath: string, targetDir?: string): Promise<{ exists: boolean; outputPath: string }> {
  const baseUrl = getApiBaseUrl()
  const params = new URLSearchParams({ sourcePath })
  if (targetDir) params.set('targetDir', targetDir)
  const response = await fetch(`${baseUrl}/api/files/encrypt-output-exists?${params}`)
  if (!response.ok) {
    console.debug('[API] checkEncryptOutputExists failed:', response.status)
    return { exists: false, outputPath: '' }
  }
  const data = await response.json()
  return { exists: !!data.exists, outputPath: data.outputPath || '' }
}

export interface FFmpegStatus {
  ffmpeg_available: boolean
  ffprobe_available: boolean
  error?: string
  ffmpeg_detail?: string
  ffprobe_detail?: string
}

export async function fetchFFmpegStatus(): Promise<FFmpegStatus> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/ffmpeg-status`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export interface BuildInfo {
  ffmpeg_version: string
  ffmpeg_codename: string
  x264_version: string
  x264_configure_opts: string
  ndk_version: string
  api_level: number
  abi: string
  build_date: string
  enabled_decoders: string[]
  enabled_encoders: string[]
  enabled_muxers: string[]
  enabled_demuxers: string[]
  enabled_parsers: string[]
  enabled_protocols: string[]
  enabled_filters: string[]
  static_libs: string[]
  linking: string
  cflags: string
  ffmpeg_license: string
  x264_license: string
  app_version?: string
}

export async function fetchBuildInfo(): Promise<BuildInfo> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/build-info`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json()
}

export interface ContainerVersionInfo {
  version: number
  status: 'deprecated' | 'stable' | 'recommended'
  label: string
}

export interface ContainerVersionsResponse {
  versions: ContainerVersionInfo[]
  default: number
}

export async function fetchContainerVersions(): Promise<ContainerVersionsResponse> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/container/versions`)
  if (!response.ok) throw new Error('Failed to fetch container versions')
  return response.json()
}

export type DecryptErrorCode = 'wrong_password' | 'data_corrupted' | 'decrypt_failed' | 'deprecated_version'

export interface DecryptError {
  error: DecryptErrorCode
  message: string
}

export function isWrongPasswordError(error: unknown): boolean {
  if (error && typeof error === 'object' && 'error' in error) {
    return (error as DecryptError).error === 'wrong_password'
  }
  const msg = String(error).toLowerCase()
  return msg.includes('wrong password') || msg.includes('密码')
}

export async function renameFile(oldPath: string, newName: string): Promise<void> {
  console.info('[API] renameFile:', oldPath, '→', newName)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/file/rename`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ oldPath, newName }),
  })
  if (!response.ok) {
    console.error('[API] renameFile failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function copyFile(srcPath: string, destPath: string): Promise<void> {
  console.info('[API] copyFile:', srcPath, '→', destPath)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/file/copy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ srcPath, destPath }),
  })
  if (!response.ok) {
    console.error('[API] copyFile failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function moveFile(srcPath: string, destPath: string): Promise<void> {
  console.info('[API] moveFile:', srcPath, '→', destPath)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/file/move`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ srcPath, destPath }),
  })
  if (!response.ok) {
    console.error('[API] moveFile failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export interface PluginMeta {
  name: string
  supportedExtensions: string[]
  supportedMimePrefixes: string[]
  containerExtension: string
  taskOptions: TaskOptions
}

export type PasswordStrategy = 'global' | 'independent' | 'none'

export interface TaskField {
  key: string
  label: string
  type: 'string' | 'password' | 'select' | 'bool'
  required: boolean
  defaultValue: string
  help: string
  options?: string[]
  condition?: '' | 'encrypt' | 'decrypt'
}

export interface TaskOptions {
  passwordStrategy: PasswordStrategy
  supportVersionSelect: boolean
  supportedVersions: number[] | null
  defaultVersion: number
  extraFields: TaskField[]
}

export interface PluginCandidate {
  name: string
  matchType: 'mime' | 'extension' | 'general' | 'container'
  priority: number
  taskOptions: TaskOptions | null
}

export interface PredictPluginResponse {
  candidates: PluginCandidate[]
  pluginName: string | null
  error?: string
  taskOptions: TaskOptions | null
}

export async function fetchPlugins(): Promise<PluginMeta[]> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/plugins`)
  if (!response.ok) {
    console.error('[API] fetchPlugins failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data = await response.json()
  console.info('[API] fetchPlugins:', data.plugins?.length || 0, 'plugins')
  return data.plugins || []
}

export interface TagInfo {
  name: string
  count: number
}

export async function fetchTags(): Promise<TagInfo[]> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files/tags`)
  if (!response.ok) {
    console.error('[API] fetchTags failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data = await response.json()
  console.info('[API] fetchTags:', data.tags?.length || 0, 'tags')
  return data.tags || []
}

export async function addTag(path: string, tag: string): Promise<void> {
  console.info('[API] addTag:', path, tag)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files/tags`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, tag, action: 'add' }),
  })
  if (!response.ok) {
    console.error('[API] addTag failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function removeTag(path: string, tag: string): Promise<void> {
  console.info('[API] removeTag:', path, tag)
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/files/tags`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, tag, action: 'remove' }),
  })
  if (!response.ok) {
    console.error('[API] removeTag failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
}

export async function listFilesByTag(tag: string, path?: string): Promise<FileItem[]> {
  const baseUrl = getApiBaseUrl()
  const params = new URLSearchParams({
    path: encodeURIComponent(path || '/'),
    tag: encodeURIComponent(tag),
  })
  const response = await fetch(`${baseUrl}/api/files?${params}`)
  if (!response.ok) {
    console.error('[API] listFilesByTag failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  const data: FileListResponse = await response.json()
  console.info('[API] listFilesByTag:', tag, '→', data.files?.length || 0, 'files')
  return data.files || []
}

export function getAlistEncryptStreamUrl(params: { path: string; password: string }): string {
  if (import.meta.env.DEV) {
    return `/api/alist-encrypt/stream?path=${encodeURIComponent(params.path)}&password=${encodeURIComponent(params.password)}`
  }
  const baseUrl = getApiBaseUrl()
  return `${baseUrl}/api/alist-encrypt/stream?path=${encodeURIComponent(params.path)}&password=${encodeURIComponent(params.password)}`
}

export interface AlistDecodeResult {
  plain_name: string
  success: boolean
}

export async function decodeAlistFilename(params: { encodedName: string; password: string; encType?: string }): Promise<AlistDecodeResult> {
  const baseUrl = getApiBaseUrl()
  const urlParams = new URLSearchParams({
    encoded: params.encodedName,
    password: params.password,
  })
  if (params.encType) urlParams.set('enc_type', params.encType)
  const response = await fetch(`${baseUrl}/api/alist-encrypt/decode-filename?${urlParams}`)
  if (!response.ok) {
    console.error('[API] decodeAlistFilename failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return response.json()
}

export interface ContainerExtensionConflict {
  extension: string
  pluginNames: string[]
}

export interface ContainerExtensionsResponse {
  extensions: Record<string, string>
  conflicts: ContainerExtensionConflict[]
}

export async function fetchContainerExtensions(): Promise<ContainerExtensionsResponse> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/plugins/container-extensions`)
  if (!response.ok) {
    console.error('[API] fetchContainerExtensions failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return response.json()
}

export async function predictPlugin(
  sourcePath: string,
  type: TaskType
): Promise<PredictPluginResponse> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks/predict-plugin`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sourcePath, type }),
  })
  if (!response.ok) {
    console.error('[API] predictPlugin failed:', response.status)
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return response.json()
}
