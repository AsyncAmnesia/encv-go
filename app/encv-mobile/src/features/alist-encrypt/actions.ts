import type { FileAction } from '@/types/file-feature'
import type { FileItem } from '@/api/encv'
import { videocam, lockClosed, lockOpen } from 'ionicons/icons'
import router from '@/router'
import { useNewTaskModal } from '@/composables/useNewTaskModal'
import { isAlistEncrypted, getStreamUrl, getDecodedName, loadDecodedName, setSessionPassword, getSessionPassword } from './useAlistEncrypt'
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
          console.error('[ALIST-DBG] stream-preview handler START: file=', f.name, 'path=', f.path)
          try {
            const password = await promptPassword(f.name)
            console.error('[ALIST-DBG] password result:', password == null ? 'CANCELLED' : ('GOT(' + password.length + 'chars)'))
            if (password == null) return
            setSessionPassword(f.path, password)
            await loadDecodedName(f, password)
            const decodedName = getDecodedName(f.path) || f.name
            const url = getStreamUrl(f, password)
            console.error('[ALIST-DBG] navigating to player: name=', decodedName, 'hasStreamUrl=', !!url)
            router.push({ path: '/player', query: { path: f.path, name: decodedName, streamUrl: url } })
          } catch (e) {
            console.error('[ALIST-DBG] stream-preview handler ERROR:', e)
          }
        },
      },
      {
        id: 'alist-decrypt',
        text: () => t('files.decrypt'),
        icon: lockClosed,
        color: 'warning',
        handler: async (f: FileItem) => {
          console.error('[ALIST-DBG] decrypt handler START: file=', f.name, 'path=', f.path)
          try {
            let password = getSessionPassword(f.path)
            console.error('[ALIST-DBG] session password:', password ? 'GOT(' + password.length + 'chars)' : 'NOT cached')
            if (!password) {
              password = await promptPassword(f.name)
              console.error('[ALIST-DBG] promptPassword result:', password == null ? 'CANCELLED' : ('GOT(' + password.length + 'chars)'))
              if (password == null) return
              setSessionPassword(f.path, password)
            }
            console.error('[ALIST-DBG] opening NewTaskModal for decrypt')
            const { openNewTask } = useNewTaskModal()
            openNewTask(f.path, 'decrypt')
          } catch (e) {
            console.error('[ALIST-DBG] decrypt handler ERROR:', e)
          }
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
