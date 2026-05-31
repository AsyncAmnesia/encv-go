import type { FileFeature } from '@/types/file-feature'
import { isAlistEncrypted, clearPasswordCache, clearDecodeCache, loadDecodedName, getDecodedName, setSessionPassword, getSessionPassword, getStreamUrl } from './useAlistEncrypt'
import { lockClosed } from 'ionicons/icons'
import { getAlistBadge } from './badge'
import { getAlistSubtitle, preloadSubtitles } from './subtitle'
import { getAlistActions } from './actions'

export function createAlistEncryptFeature(): FileFeature {
  return {
    id: 'alist-encrypt',
    isActive: (file) => !file.isDirectory,
    getBadge: (file) => getAlistBadge(file),
    getSubtitle: (file) => getAlistSubtitle(file),
    getFileActions: (file) => getAlistActions(file),
    isContainerFile: (file) => isAlistEncrypted(file),
    handleClick: (file): { handled: true } | null => {
      if (!isAlistEncrypted(file)) return null
      return { handled: true }
    },
    icon: lockClosed,
    onActivate() {
      console.info('[alist-encrypt] Feature activated')
    },
    onDeactivate() {
      clearPasswordCache()
      clearDecodeCache()
    },
  }
}

export { preloadSubtitles }
export { isAlistEncrypted, loadDecodedName, getDecodedName, setSessionPassword, getSessionPassword, getStreamUrl }
export { promptPassword } from './password-dialog'
