import { createRouter, createWebHashHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'

import OpenListHome from '@/views/OpenListHome.vue'
import OpenListConfigEditor from '@/views/OpenListConfigEditor.vue'
import OpenListSettings from '@/views/OpenListSettings.vue'
import OpenListWebView from '@/views/OpenListWebView.vue'
import BackToMain from '@/views/BackToMain.vue'

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/home' },
  { path: '/home', component: OpenListHome },
  { path: '/config', component: OpenListConfigEditor },
  { path: '/settings', component: OpenListSettings },
  { path: '/webview', component: OpenListWebView },
  // "返回 ENCV 主页面"视图：内嵌全屏 iframe 加载 encv-mobile :5173，
  // 绕过 Trae 沙箱 OpenPreview 工具「单 port 限制」(trae_web_sandbox_network.md §8.4)
  { path: '/back-to-main', component: BackToMain },
]

/**
 * 必须用 hash 模式（createWebHashHistory）！
 * 原因：Android WebView 通过 file:///android_asset/openlist/index.html 加载
 * - file:// 协议不支持 history.pushState
 * - 即使支持，刷新非根路径也会 404（无服务端路由）
 * - hash 模式（#/home）天然兼容 file:// + 刷新友好
 */
export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})
