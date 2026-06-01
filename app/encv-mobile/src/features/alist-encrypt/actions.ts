import type { FileAction } from '@/types/file-feature'
import type { FileItem } from '@/api/encv'
import { videocam, lockClosed, lockOpen } from 'ionicons/icons'
import router from '@/router'
import { useNewTaskModal } from '@/composables/useNewTaskModal'
import { isAlistEncrypted, getStreamUrl, getDecodedName, loadDecodedName, setSessionPassword } from './useAlistEncrypt'
import { promptPassword } from './password-dialog'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

export function getAlistActions(file: FileItem): FileAction[] {
  if (isAlistEncrypted(file)) {
    return [
      {
        id: 'alist-stream-preview',
        text: () => t('alistEncrypt.streamPreview'),
        icon: videocam,
        color: 'primary',
        handler: async (f: FileItem) => {
          const password = await promptPassword(f.name)
          if (password == null) return
          setSessionPassword(f.path, password)
          await loadDecodedName(f, password)
          const decodedName = getDecodedName(f.path) || f.name
          const url = getStreamUrl(f, password)
          router.push({ path: '/player', query: { path: f.path, name: decodedName, streamUrl: url } })
        },
      },
      {
        id: 'alist-decrypt',
        text: () => t('files.decrypt'),
        icon: lockClosed,
        color: 'warning',
        handler: async (f: FileItem) => {
          const { openNewTask } = useNewTaskModal()
          openNewTask(f.path, 'decrypt')
        },
      },
    ]
  }

  if (file.isEncrypted === true) {
    return [
      {
        id: 'alist-decrypt-container',
        text: () => t('files.decrypt'),
        icon: lockOpen,
        color: 'primary',
        handler: async (f: FileItem) => {
          const { openNewTask } = useNewTaskModal()
          openNewTask(f.path, 'decrypt')
        },
      },
    ]
  }

  return [
    {
      id: 'alist-encrypt',
      text: () => t('files.encrypt'),
      icon: lockClosed,
      color: 'warning',
      handler: async (f: FileItem) => {
        const { openNewTask } = useNewTaskModal()
        openNewTask(f.path, 'encrypt')
      },
    },
  ]
}
