import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'
import Tabs from '@/views/Tabs.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/tabs/home',
  },
  {
    path: '/player',
    component: () => import('@/views/ArtPlayerView.vue'),
  },
  {
    path: '/tabs/',
    component: Tabs,
    children: [
      {
        path: '',
        redirect: '/tabs/home',
      },
      {
        path: 'home',
        component: () => import('@/views/HomePage.vue'),
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
        path: 'extensions',
        component: () => import('@/views/ExtensionsPage.vue'),
        meta: { title: 'extensions.title' },
      },
      {
        path: 'settings/server',
        component: () => import('@/views/ServerDetail.vue'),
      },
      {
        path: 'settings/engine',
        component: () => import('@/views/EngineDetail.vue'),
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
      {
        path: 'file-info',
        component: () => import('@/views/FileInfo.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
