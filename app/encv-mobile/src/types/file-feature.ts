import type { FileItem } from '@/api/encv'

export interface FileAction {
  id: string
  text: () => string
  icon: any
  color?: 'primary' | 'danger' | 'warning'
  visible?: (file: FileItem) => boolean
  handler: (file: FileItem) => Promise<void>
}

export interface FileBadge {
  text: string
  color: string
  icon?: any
}

export interface FileSubtitle {
  text: string
  color?: string
}

export interface FileFeature {
  id: string
  isActive(file: FileItem): boolean
  getBadge?(file: FileItem): FileBadge | null | Promise<FileBadge | null>
  getSubtitle?(file: FileItem): FileSubtitle | null | Promise<FileSubtitle | null>
  getFileActions?(file: FileItem): FileAction[] | Promise<FileAction[]>
  onActivate?(): void
  onDeactivate?(): void
}
