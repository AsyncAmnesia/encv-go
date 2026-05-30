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
        path: 'settings/devtools',
        component: () => import('@/views/DevToolsDetail.vue'),
      },
      {
        path: 'settings/devtools/prototype/:id',
        component: () => import('@/views/PrototypeSandbox.vue'),
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

router.beforeEach((to, from) => {
  console.error('[SAT-DBG][Router] beforeEach |', from.path, '→', to.path, '| ts=', Date.now())
})

router.afterEach((to, from) => {
  console.error('[SAT-DBG][Router] afterEach  |', from.path, '→', to.path, '| ts=', Date.now())
})

router.onError((error) => {
  console.error('[SAT-DBG][Router] onError |', error, '| ts=', Date.now())
})

export default router
