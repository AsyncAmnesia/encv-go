import type { Component } from 'vue'

export interface PrototypeDefinition {
  id: string
  name: string
  route: string
  composePath: string
  description: string
  icon: string
  accentColor: string
  component: () => Promise<{ default: Component }>
  composeSource: () => Promise<string>
  webSource: () => Promise<string>
}

const prototypes: PrototypeDefinition[] = [
  {
    id: 'mpv-player-screen',
    name: 'MPV Player Screen',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvPlayerScreen',
    description: 'MPV 播放器主界面，包含视频渲染区和控制层叠加',
    icon: 'play-circle',
    accentColor: 'rgba(139, 92, 246, 0.15)',
    component: () => import('./MpvPlayerPrototype.vue'),
    composeSource: () => import('./sources/mpv-player-screen-compose.txt?raw').then(m => m.default),
    webSource: () => import('./sources/mpv-player-screen-web.html?raw').then(m => m.default),
  },
  {
    id: 'mpv-controls',
    name: 'MPV Controls Overlay',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvControls',
    description: '播放控制叠加层：播放/暂停、进度条、音量、全屏切换',
    icon: 'settings',
    accentColor: 'rgba(56, 128, 255, 0.15)',
    component: () => import('./MpvPlayerPrototype.vue'),
    composeSource: () => import('./sources/mpv-controls-compose.txt?raw').then(m => m.default),
    webSource: () => import('./sources/mpv-controls-web.html?raw').then(m => m.default),
  },
  {
    id: 'mpv-progress-bar',
    name: 'MPV Progress Bar',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvProgressBar',
    description: '可拖拽进度条组件，含时间显示和缓冲指示',
    icon: 'musical-notes',
    accentColor: 'rgba(45, 211, 111, 0.15)',
    component: () => import('./MpvPlayerPrototype.vue'),
    composeSource: () => import('./sources/mpv-progress-bar-compose.txt?raw').then(m => m.default),
    webSource: () => import('./sources/mpv-progress-bar-web.html?raw').then(m => m.default),
  },
  {
    id: 'mpv-theme',
    name: 'MPV Theme (EncvMpVPlayerTheme)',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.theme.EncvMpVPlayerTheme',
    description: 'EncvMpVPlayerTheme 主题配色系统，定义暗色播放器色彩方案',
    icon: 'color-palette',
    accentColor: 'rgba(235, 68, 90, 0.15)',
    component: () => import('./MpvPlayerPrototype.vue'),
    composeSource: () => import('./sources/mpv-theme-compose.txt?raw').then(m => m.default),
    webSource: () => import('./sources/mpv-theme-web.html?raw').then(m => m.default),
  },
]

export function getPrototype(id: string): PrototypeDefinition | undefined {
  return prototypes.find(p => p.id === id)
}

export function getAllPrototypes(): PrototypeDefinition[] {
  return prototypes
}
