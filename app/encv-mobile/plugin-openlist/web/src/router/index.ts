import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'

import OpenListHome from '@/views/OpenListHome.vue'
import OpenListConfigEditor from '@/views/OpenListConfigEditor.vue'
import OpenListSettings from '@/views/OpenListSettings.vue'
import OpenListWebView from '@/views/OpenListWebView.vue'

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/home' },
  { path: '/home', component: OpenListHome },
  { path: '/config', component: OpenListConfigEditor },
  { path: '/settings', component: OpenListSettings },
  { path: '/webview', component: OpenListWebView },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
