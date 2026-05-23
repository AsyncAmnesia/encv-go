import { createRouter, createMemoryHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import PlayerView from './views/PlayerView.vue'
import PlaylistView from './views/PlaylistView.vue'
import SettingsView from './views/SettingsView.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/player', name: 'player', component: PlayerView },
    { path: '/playlist', name: 'playlist', component: PlaylistView },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})

export default router
