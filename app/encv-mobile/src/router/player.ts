import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/player',
  },
  {
    path: '/player',
    component: () => import('@/views/StandalonePlayer.vue'),
  },
  {
    path: '/player/settings',
    component: () => import('@/views/PlayerSettings.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
