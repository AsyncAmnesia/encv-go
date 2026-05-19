import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'
import Tabs from '@/views/Tabs.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/tabs/files',
  },
  {
    path: '/tabs/',
    component: Tabs,
    children: [
      {
        path: '',
        redirect: '/tabs/files',
      },
      {
        path: 'files',
        component: () => import('@/views/Files.vue'),
      },
      {
        path: 'player',
        component: () => import('@/views/Player.vue'),
      },
      {
        path: 'tasks',
        component: () => import('@/views/Tasks.vue'),
      },
      {
        path: 'webdav',
        component: () => import('@/views/WebDAV.vue'),
      },
      {
        path: 'settings',
        component: () => import('@/views/Settings.vue'),
      },
      {
        path: 'devlogs',
        component: () => import('@/views/DevLogs.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
