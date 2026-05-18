const API_BASE_URL = 'http://127.0.0.1:2025'

export async function listFiles(path = '/') {
  try {
    const response = await fetch(`${API_BASE_URL}/api/files?path=${encodeURIComponent(path)}`)
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }
    return await response.json()
  } catch (error) {
    console.error('Error listing files:', error)
    throw error
  }
}

export async function getFileStream(path) {
  return `${API_BASE_URL}/stream?path=${encodeURIComponent(path)}`
}

export async function checkServerStatus() {
  try {
    const response = await fetch(`${API_BASE_URL}/health`)
    return response.ok
  } catch (error) {
    return false
  }
}
