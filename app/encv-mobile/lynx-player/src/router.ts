import { createRouter, createMemoryHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import PlayerView from './views/PlayerView.vue'
import PlaylistView from './views/PlaylistView.vue'
import SettingsView from './views/SettingsView.vue'

function getInitialRoute(): string {
  try {
    const lynxObj = (globalThis as any).lynx
    const globalProps = lynxObj?.__globalProps
    if (globalProps?.filePath) {
      return '/player'
    }
  } catch (_e) {
    // ignore
  }
  return '/'
}

const router = createRouter({
  history: createMemoryHistory(getInitialRoute()),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/player', name: 'player', component: PlayerView },
    { path: '/playlist', name: 'playlist', component: PlaylistView },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})

export default router
