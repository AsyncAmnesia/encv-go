window.addEventListener('error', (e) => {
  console.error('[PLAYER-ERROR]', e.message, e.filename, e.lineno, e.colno, e.error)
})
window.addEventListener('unhandledrejection', (e) => {
  console.error('[PLAYER-PROMISE-REJECT]', e.reason)
})
console.log('[PLAYER-INIT] player-main.ts starting')

import { createApp } from 'vue'
import { IonicVue } from '@ionic/vue'
import PlayerApp from '@/PlayerApp.vue'
import playerRouter from '@/router/player'

import '@ionic/vue/css/core.css'
import '@ionic/vue/css/normalize.css'
import './theme/variables.css'

const app = createApp(PlayerApp).use(IonicVue).use(playerRouter)

playerRouter.isReady().then(() => {
  app.mount('#app')
})
