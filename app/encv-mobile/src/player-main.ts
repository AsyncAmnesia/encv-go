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
