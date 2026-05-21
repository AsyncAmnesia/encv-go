window.addEventListener('error', (e) => {
  console.error('[PLAYER-ERROR]', e.message, e.filename, e.lineno, e.colno, e.error)
})
window.addEventListener('unhandledrejection', (e) => {
  console.error('[PLAYER-PROMISE-REJECT]', e.reason)
})
console.log('[PLAYER-INIT] player-main.ts starting')
console.log('[PLAYER-INIT] URL:', location.href)
console.log('[PLAYER-INIT] Capacitor:', typeof Capacitor !== 'undefined' ? 'exists' : 'MISSING')
console.log('[PLAYER-INIT] Capacitor.isNativePlatform():', typeof Capacitor !== 'undefined' ? Capacitor?.isNativePlatform() : 'N/A')
if (typeof Capacitor !== 'undefined') {
  console.log('[PLAYER-INIT] Capacitor.platform:', Capacitor?.platform)
  console.log('[PLAYER-INIT] Capacitor.Plugins keys:', Object.keys(Capacitor?.Plugins || {}))
}

import { createApp } from 'vue'
import { IonicVue } from '@ionic/vue'
import PlayerApp from '@/PlayerApp.vue'
import playerRouter from '@/router/player'

import '@ionic/vue/css/core.css'
import '@ionic/vue/css/normalize.css'
import './theme/variables.css'

const app = createApp(PlayerApp).use(IonicVue).use(playerRouter)

playerRouter.isReady().then(() => {
  console.log('[PLAYER-INIT] Router ready, mounting app to #app')
  const el = document.getElementById('app')
  console.log('[PLAYER-INIT] #app element:', !!el, 'innerHTML length:', el?.innerHTML?.length || 0)
  app.mount('#app')
  console.log('[PLAYER-INIT] App mounted successfully')
})
