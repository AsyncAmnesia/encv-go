import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

/**
 * plugin-openlist/web Vite 配置
 *
 * 职责单一：仅服务 plugin 管理 UI（OpenListHome / OpenListSettings / OpenListConfigEditor / OpenListWebView）
 *
 * 不再做 OpenList 后端代理。
 * 撤销原因：subpath 改造（/openlist-spa/、/openlist-ui/）不可靠，OpenList 应在原始环境 / 跑。
 * 沙箱 dev 直接通过 http://127.0.0.1:5244/ 访问 OpenList 真实前端（CORS=*），与 prod 模式对齐。
 *
 * 沙箱 dev 启动 OpenList 后端（真实 Hi-Sillot fork）：
 *   Terminal 1: bash scripts/dev-openlist.sh
 *   → 启动 http://127.0.0.1:5244/，前端 dist 来自 app/openlist/Hi-Sillot-OpenList/public/dist/
 *
 * 沙箱 dev 启动本 Vite（plugin 管理 UI，端口 5174）：
 *   Terminal 2: bash scripts/dev-openlist-web.sh
 *   → OpenListWebView 内的 iframe 直访 http://127.0.0.1:5244/#/login
 *
 * Production（Android WebView）：
 *   - WebView 加载 file:///android_asset/openlist/index.html（plugin-openlist/src/main/assets/openlist/）
 *   - iframe 内部直访 http://127.0.0.1:5244/（与本机 OpenList 进程同设备）
 *
 * 与主 app encv-mobile Vite (8100) 的 /openlist-ui-proxy 无关：
 *   - 那个是主 app 开发期「在浏览器里调试 encv-mobile + 顺便看 OpenList」用的辅助中间件
 *   - plugin-openlist/web 走的是「对齐 prod 的正式路径」，两者职责互不重叠
 */

export default defineConfig({
  // ⚠️ 必须 './'：Android WebView 通过 file:///android_asset/openlist/ 加载
  // 绝对路径 '/assets/...' 在 file:// 协议下 404
  base: './',
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    // 显式清空 dist（保证 CI 干净构建）
    emptyOutDir: true,
  },
  server: {
    port: 5174,
    strictPort: false,
  },
})
