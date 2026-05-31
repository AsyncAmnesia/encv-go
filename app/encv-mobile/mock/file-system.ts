import type { FileItem } from '@/api/encv'

export interface MockFileItem extends FileItem {}

let MOCK_SUFFIX = '.ae'

export function setMockSuffix(suffix: string): void {
  MOCK_SUFFIX = suffix
}

export function getMockSuffix(): string {
  return MOCK_SUFFIX
}

let customFiles: Map<string, MockFileItem[]> = new Map()

export function setMockFiles(path: string, files: MockFileItem[]): void {
  customFiles.set(path, files)
}

export function addMockFile(path: string, file: MockFileItem): void {
  const existing = customFiles.get(path) || []
  if (!customFiles.has(path)) {
    customFiles.set(path, [...existing])
  }
  customFiles.get(path)!.push(file)
}

export function removeMockFile(path: string, fileName: string): void {
  const files = customFiles.get(path)
  if (files) {
    const idx = files.findIndex(f => f.name === fileName)
    if (idx !== -1) files.splice(idx, 1)
  }
}

export function resetMockFiles(): void {
  customFiles.clear()
}

export const MOCK_PLUGINS = [
  {
    name: 'video',
    supportedExtensions: ['mp4', 'mkv', 'avi', 'mov', 'sccgv'],
    supportedMimePrefixes: ['video/'],
    containerExtension: '.sccgv',
    taskOptions: {
      passwordStrategy: 'global' as const,
      supportVersionSelect: true,
      supportedVersions: [1, 2, 3],
      defaultVersion: 2,
      extraFields: [],
    },
  },
  {
    name: 'image',
    supportedExtensions: ['jpg', 'jpeg', 'png', 'gif', 'webp', 'sccgi'],
    supportedMimePrefixes: ['image/'],
    containerExtension: '.sccgi',
    taskOptions: {
      passwordStrategy: 'global' as const,
      supportVersionSelect: true,
      supportedVersions: [1, 2],
      defaultVersion: 1,
      extraFields: [],
    },
  },
  {
    name: 'audio',
    supportedExtensions: ['mp3', 'flac', 'wav', 'aac', 'sccga'],
    supportedMimePrefixes: ['audio/'],
    containerExtension: '.sccga',
    taskOptions: {
      passwordStrategy: 'independent' as const,
      supportVersionSelect: false,
      supportedVersions: null,
      defaultVersion: 1,
      extraFields: [],
    },
  },
  {
    name: 'text',
    supportedExtensions: ['txt', 'md', 'csv', 'sccgt'],
    supportedMimePrefixes: ['text/'],
    containerExtension: '.sccgt',
    taskOptions: {
      passwordStrategy: 'none' as const,
      supportVersionSelect: false,
      supportedVersions: null,
      defaultVersion: 1,
      extraFields: [],
    },
  },
  {
    name: 'wps',
    supportedExtensions: ['doc', 'docx', 'xls', 'xlsx', 'sccgwps'],
    supportedMimePrefixes: [],
    containerExtension: '.sccgwps',
    taskOptions: {
      passwordStrategy: 'independent' as const,
      supportVersionSelect: true,
      supportedVersions: [1],
      defaultVersion: 1,
      extraFields: [
        { key: 'format', label: '输出格式', type: 'select', required: true, defaultValue: 'docx', help: '选择输出格式', options: ['docx', 'pdf'] },
      ],
    },
  },
  {
    name: 'pdf',
    supportedExtensions: ['pdf', 'sccgpdf'],
    supportedMimePrefixes: ['application/pdf'],
    containerExtension: '.sccgpdf',
    taskOptions: {
      passwordStrategy: 'independent' as const,
      supportVersionSelect: false,
      supportedVersions: null,
      defaultVersion: 1,
      extraFields: [],
    },
  },
]
