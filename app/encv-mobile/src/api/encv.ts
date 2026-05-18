const API_BASE_URL = 'http://127.0.0.1:2025'

interface FileItem {
  name: string
  path: string
  isDirectory: boolean
  size?: number
  modified?: string
}

interface FileListResponse {
  files: FileItem[]
}

export async function listFiles(path = '/'): Promise<FileListResponse> {
  const response = await fetch(`${API_BASE_URL}/api/files?path=${encodeURIComponent(path)}`)
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  return await response.json() as FileListResponse
}

export function getFileStreamUrl(path: string): string {
  return `${API_BASE_URL}/stream?path=${encodeURIComponent(path)}`
}

export async function checkServerStatus(): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE_URL}/health`)
    return response.ok
  } catch {
    return false
  }
}
