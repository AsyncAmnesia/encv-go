import { ref } from 'vue'
import common from '@/i18n/common'
import tasks from '@/i18n/tasks'
import files from '@/i18n/files'
import player from '@/i18n/player'
import settings from '@/i18n/settings'
import devlogs from '@/i18n/devlogs'
import extensions from '@/i18n/extensions'
import errors from '@/i18n/errors'
import modals from '@/i18n/modals'

type Locale = 'zh-CN' | 'en'
type MessageModule = { 'zh-CN': Record<string, string>; en: Record<string, string> }

function mergeModules(modules: MessageModule[]): Record<Locale, Record<string, string>> {
  const merged: Record<Locale, Record<string, string>> = { 'zh-CN': {}, en: {} }
  for (const mod of modules) {
    merged['zh-CN'] = { ...merged['zh-CN'], ...mod['zh-CN'] }
    merged.en = { ...merged.en, ...mod.en }
  }
  return merged
}

const messages: Record<Locale, Record<string, string>> = mergeModules([
  common,
  tasks,
  files,
  player,
  settings,
  devlogs,
  extensions,
  errors,
  modals,
])

function getStoredLocale(): Locale {
  const stored = localStorage.getItem('encv-locale')
  if (stored === 'en' || stored === 'zh-CN') return stored
  return 'zh-CN'
}

const currentLocale = ref<Locale>(getStoredLocale())

const fieldKeyMap: Record<string, string> = {
  'password': 'settings.password',
  'recover': 'settings.recover',
  'output_path': 'settings.outputPath',
  'plugin_settings': 'settings.pluginSettings',
  'server': 'settings.httpServerSettings',
  'admin': 'settings.adminServerSettings',
  'webdav': 'settings.webdavServerSettings',
  'proxy': 'settings.proxyServerSettings',
  'log': 'settings.logSettings',
  'port': 'settings.port',
  'dir': 'settings.dir',
  'username': 'settings.username',
  'root': 'settings.root',
  'level': 'settings.level',
  'file': 'settings.file',
  'host': 'settings.host',
  'description': 'settings.description',
  'sites': 'settings.sites',
  'disable_signature_verification': 'settings.disableSignatureVerification',
  'ext': 'settings.ext',
  'chunk_size_mb': 'settings.chunkSizeMb',
  'light_main_chunk_enabled': 'settings.lightMainChunkEnabled',
  'track_extensions': 'settings.trackExtensions',
  'keep_mkv_for_mkv_source': 'settings.keepMkvForMkvSource',
  'verify_after_pack': 'settings.verifyAfterPack',
  'plugin_cache_dir': 'settings.pluginCacheDir',
  'skip_merge_for_split_mkv': 'settings.skipMergeForSplitMkv',
  'video': 'settings.video',
  'audio': 'settings.audio',
  'image': 'settings.image',
  'wps': 'settings.wps',
  'pdf': 'settings.pdf',
  'text': 'settings.text',
  'custom_text_extensions': 'settings.customTextExts',
}

const sectionTitleMap: Record<string, string> = {
  '全局设置': 'settings.globalSettings',
  '加密/解密设置': 'settings.encryptDecryptSettings',
  '内置HTTP服务器 设置': 'settings.httpServerSettings',
  '管理后台服务器 设置': 'settings.adminServerSettings',
  'WebDAV 服务器设置': 'settings.webdavServerSettings',
  'Openlist 代理服务器设置': 'settings.proxyServerSettings',
  '日志设置': 'settings.logSettings',
}

function t(key: string, params?: Record<string, string>): string {
  let msg = messages[currentLocale.value]?.[key] || key
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      msg = msg.replace(`{${k}}`, v)
    }
  }
  return msg
}

function tField(key: string): string {
  const i18nKey = fieldKeyMap[key]
  if (i18nKey) return t(i18nKey)
  return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

function tSectionTitle(title: string): string {
  const i18nKey = sectionTitleMap[title]
  if (i18nKey) return t(i18nKey)
  return title
}

function setLocale(locale: Locale) {
  currentLocale.value = locale
  localStorage.setItem('encv-locale', locale)
}

function getLocale(): Locale {
  return currentLocale.value
}

export function useI18n() {
  return {
    t,
    tField,
    tSectionTitle,
    setLocale,
    getLocale,
    locale: currentLocale,
  }
}
