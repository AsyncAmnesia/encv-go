import type { Ref } from 'vue'
import type { FileItem } from '@/api/encv'

export interface MockFileItem {
  name: string
  path: string
  isDirectory: boolean
  isEncrypted?: boolean
  size?: number
  modified?: string
}

export interface TestBackdoorAPI {
  simulateLongPress: (fileName: string) => Promise<void>
  simulateFileClick: (fileName: string) => Promise<void>
  getMockFiles: () => Promise<MockFileItem[]>
  setMockFiles: (path: string, files: MockFileItem[]) => Promise<void>
  addMockFile: (path: string, file: MockFileItem) => Promise<void>
  removeMockFile: (path: string, name: string) => Promise<void>
  navigateToPath: (path: string) => void
  getCurrentFiles: () => FileItem[]
  triggerActionSheet: (fileName: string) => Promise<void>
  openNewTaskModal: (sourcePath?: string, taskType?: 'encrypt' | 'decrypt') => Promise<void>
}

declare global {
  interface Window {
    __ENCV_TEST?: TestBackdoorAPI
  }
}

async function mockControl(action: string, params: Record<string, string> = {}): Promise<any> {
  const qs = new URLSearchParams({ action, ...params }).toString()
  const res = await fetch(`/__mock_control?${qs}`)
  if (!res.ok) throw new Error(`[TEST-BACKDOOR] mock control failed: ${res.status}`)
  return res.json()
}

export function useTestBackdoor(
  files: Ref<FileItem[]>,
  options: {
    onLongPress: (file: FileItem) => Promise<void>
    onClick: (file: FileItem) => Promise<void>
    navigateTo: (path: string) => void
    openNewTask?: (sourcePath?: string, taskType?: 'encrypt' | 'decrypt') => Promise<void>
  }
): TestBackdoorAPI | null {
  if (!import.meta.env.DEV) return null

  const api: TestBackdoorAPI = {
    simulateLongPress: async (fileName: string) => {
      const file = files.value.find(f => f.name === fileName)
      if (!file) throw new Error(`[TEST-BACKDOOR] File not found: ${fileName}`)
      console.warn(`[TEST-BACKDOOR] simulateLongPress(${fileName})`)
      await options.onLongPress(file)
    },

    simulateFileClick: async (fileName: string) => {
      const file = files.value.find(f => f.name === fileName)
      if (!file) throw new Error(`[TEST-BACKDOOR] File not found: ${fileName}`)
      console.warn(`[TEST-BACKDOOR] simulateFileClick(${fileName})`)
      await options.onClick(file)
    },

    getMockFiles: () => {
      return mockControl('get_files', { path: '/' })
    },

    setMockFiles: (path: string, fileList: MockFileItem[]) => {
      console.warn(`[TEST-BACKDOOR] setMockFiles(${path}, ${fileList.length} files)`)
      return mockControl('set_files', { path, file: JSON.stringify(fileList) })
    },

    addMockFile: (path: string, file: MockFileItem) => {
      console.warn(`[TEST-BACKDOOR] addMockFile(${path}, ${file.name})`)
      return mockControl('add_file', { path, file: JSON.stringify(file) })
    },

    removeMockFile: (path: string, name: string) => {
      console.warn(`[TEST-BACKDOOR] removeMockFile(${path}, ${name})`)
      return mockControl('remove_file', { path, name })
    },

    navigateToPath: (path: string) => {
      console.warn(`[TEST-BACKDOOR] navigateToPath(${path})`)
      options.navigateTo(path)
    },

    getCurrentFiles: () => {
      return [...files.value]
    },

    triggerActionSheet: async (fileName: string) => {
      return api.simulateLongPress(fileName)
    },

    openNewTaskModal: async (sourcePath?: string, taskType?: 'encrypt' | 'decrypt') => {
      if (!options.openNewTask) {
        throw new Error('[TEST-BACKDOOR] openNewTask not provided in options')
      }
      console.warn(`[TEST-BACKDOOR] openNewTaskModal(${sourcePath}, ${taskType})`)
      await options.openNewTask(sourcePath, taskType)
    },
  }

  window.__ENCV_TEST = api
  console.warn('[TEST-BACKDOOR] API registered on window.__ENCV_TEST')
  console.warn('[TEST-BACKDOOR] Available methods:', Object.keys(api).join(', '))

  return api
}
