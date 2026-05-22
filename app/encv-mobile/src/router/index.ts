import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'
import Tabs from '@/views/Tabs.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/tabs/files',
  },
  {
    path: '/player',
    component: () => import('@/views/StandalonePlayer.vue'),
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
        path: 'tasks',
        component: () => import('@/views/Tasks.vue'),
      },
      {
        path: 'remote',
        component: () => import('@/views/Remote.vue'),
      },
      {
        path: 'settings',
        component: () => import('@/views/Settings.vue'),
      },
      {
        path: 'settings/server',
        component: () => import('@/views/ServerDetail.vue'),
      },
      {
        path: 'settings/about',
        component: () => import('@/views/AboutDetail.vue'),
      },
      {
        path: 'settings/cache',
        component: () => import('@/views/CacheDetail.vue'),
      },
      {
        path: 'settings/plugins',
        component: () => import('@/views/PluginSettings.vue'),
      },
      {
        path: 'devlogs',
        component: () => import('@/views/DevLogs.vue'),
      },
      {
        path: 'preview',
        component: () => import('@/views/FilePreview.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
