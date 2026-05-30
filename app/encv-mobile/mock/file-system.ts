import type { FileItem } from '@/api/encv'

export interface MockFileItem extends FileItem {}

let MOCK_SUFFIX = '.ae'

export function setMockSuffix(suffix: string): void {
  MOCK_SUFFIX = suffix
}

export function getMockSuffix(): string {
  return MOCK_SUFFIX
}

const DEFAULT_FILES: MockFileItem[] = [
  { name: 'sample.mp4', path: '/sample.mp4', isDirectory: false, size: 15728640, modified: '2026-05-30T10:00:00Z' },
  { name: 'movie.mkv', path: '/movie.mkv', isDirectory: false, size: 1048576000, modified: '2026-05-29T18:30:00Z' },
  { name: 'music.mp3', path: '/music.mp3', isDirectory: false, size: 8388608, modified: '2026-05-28T14:20:00Z' },
  { name: 'podcast.flac', path: '/podcast.flac', isDirectory: false, size: 31457280, modified: '2026-05-27T09:15:00Z' },
  { name: 'photo.jpg', path: '/photo.jpg', isDirectory: false, size: 2097152, modified: '2026-05-26T16:45:00Z' },
  { name: 'screenshot.png', path: '/screenshot.png', isDirectory: false, size: 5242880, modified: '2026-05-25T11:00:00Z' },
  { name: 'report.pdf', path: '/report.pdf', isDirectory: false, size: 1048576, modified: '2026-05-24T08:30:00Z' },
  { name: 'notes.txt', path: '/notes.txt', isDirectory: false, size: 2048, modified: '2026-05-23T22:10:00Z' },
  { name: 'data.csv', path: '/data.csv', isDirectory: false, size: 4096, modified: '2026-05-22T13:00:00Z' },
  { name: `secret${MOCK_SUFFIX}`, path: `/secret${MOCK_SUFFIX}`, isDirectory: false, size: 5242880, modified: '2026-05-30T12:00:00Z' },
  { name: `document${MOCK_SUFFIX}`, path: `/document${MOCK_SUFFIX}`, isDirectory: false, size: 1048576, modified: '2026-05-29T15:00:00Z' },
  { name: 'video.sccgv', path: '/video.sccgv', isDirectory: false, size: 52428800, modified: '2026-05-30T08:00:00Z', isEncrypted: true },
  { name: 'image.sccgi', path: '/image.sccgi', isDirectory: false, size: 2097152, modified: '2026-05-29T20:00:00Z', isEncrypted: true },
  { name: 'audio.sccga', path: '/audio.sccga', isDirectory: false, size: 4194304, modified: '2026-05-28T12:00:00Z', isEncrypted: true },
  { name: 'Movies', path: '/Movies', isDirectory: true, modified: '2026-05-30T07:00:00Z' },
  { name: 'Documents', path: '/Documents', isDirectory: true, modified: '2026-05-30T07:00:00Z' },
  { name: 'Music', path: '/Music', isDirectory: true, modified: '2026-05-30T07:00:00Z' },
]

const MOVIES_FILES: MockFileItem[] = [
  { name: 'action-movie.sccgv', path: '/Movies/action-movie.sccgv', isDirectory: false, size: 2147483648, modified: '2026-05-29T10:00:00Z', isEncrypted: true },
  { name: 'comedy.mkv', path: '/Movies/comedy.mkv', isDirectory: false, size: 1073741824, modified: '2026-05-28T18:00:00Z' },
  { name: `hidden-gem${MOCK_SUFFIX}`, path: `/Movies/hidden-gem${MOCK_SUFFIX}`, isDirectory: false, size: 8388608, modified: '2026-05-27T14:00:00Z' },
]

const DOCUMENTS_FILES: MockFileItem[] = [
  { name: 'contract.pdf', path: '/Documents/contract.pdf', isDirectory: false, size: 512000, modified: '2026-05-30T09:00:00Z' },
  { name: 'memo.txt', path: '/Documents/memo.txt', isDirectory: false, size: 1024, modified: '2026-05-29T11:00:00Z' },
  { name: `confidential${MOCK_SUFFIX}`, path: `/Documents/confidential${MOCK_SUFFIX}`, isDirectory: false, size: 204800, modified: '2026-05-28T16:00:00Z' },
  { name: 'archive.sccgpdf', path: '/Documents/archive.sccgpdf', isDirectory: false, size: 1024000, modified: '2026-05-27T10:00:00Z', isEncrypted: true },
]

const MUSIC_FILES: MockFileItem[] = [
  { name: 'song1.mp3', path: '/Music/song1.mp3', isDirectory: false, size: 5242880, modified: '2026-05-30T08:00:00Z' },
  { name: 'song2.flac', path: '/Music/song2.flac', isDirectory: false, size: 20971520, modified: '2026-05-29T20:00:00Z' },
  { name: `album-secret${MOCK_SUFFIX}`, path: `/Music/album-secret${MOCK_SUFFIX}`, isDirectory: false, size: 3145728, modified: '2026-05-28T12:00:00Z' },
  { name: 'track.sccga', path: '/Music/track.sccga', isDirectory: false, size: 6291456, modified: '2026-05-27T09:00:00Z', isEncrypted: true },
]

const FILE_MAP: Record<string, MockFileItem[]> = {
  '/': DEFAULT_FILES,
  '/Movies': MOVIES_FILES,
  '/Documents': DOCUMENTS_FILES,
  '/Music': MUSIC_FILES,
}

let customFiles: Map<string, MockFileItem[]> = new Map()

export function getFiles(path: string): MockFileItem[] {
  if (customFiles.has(path)) return customFiles.get(path)!
  return FILE_MAP[path] || []
}

export function setMockFiles(path: string, files: MockFileItem[]): void {
  customFiles.set(path, files)
}

export function addMockFile(path: string, file: MockFileItem): void {
  const existing = customFiles.get(path) || getFiles(path)
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
