import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// 注意：OpenList UI 的静态服务由谁承担，取决于运行模式：
//   - dev 沙箱预览：Hi-Sillot-OpenList-Frontend 的 vite dev server (:3000)，
//     以 OPENLIST_PREVIEW_BASE="/openlist-ui/" 启动。Vite 透传 + dynamic base 改写 index.html。
//     配置：app/openlist/Hi-Sillot-OpenList-Frontend/vite.config.ts 读 OPENLIST_PREVIEW_BASE。
//   - 生产：APK 内 gomobile 进程从 filesDir/openlist/dist/ 自己服务。
// 前端入口仍是后端驱动：GET /api/preview/openlist-ui → 302 → /openlist-ui/
// 见：internal/server/openlist_ui_handler.go handlePreviewRedirect。

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 8100,
    // Listen on all interfaces so OpenPreview / external previews can reach
    // the Vite dev server (default is localhost-only which is IPv6-only on
    // some sandboxes, breaking IPv4 / hostname access).
    host: '0.0.0.0',
    // Allow reading app/openlist/ (parent of encv-mobile/) so Vite can serve the fork's dist
    fs: {
      allow: [path.resolve(__dirname, '..')],
    },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      '/p': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      // /openlist/sites/* — encv-go reverse proxy for the runtime data path.
      // NOTE: prefix MUST be `/openlist/` (with trailing slash) to avoid
      // hijacking `/openlist-ui/*` (which goes through the line below).
      '/openlist/': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      // /openlist-ui/* — 透传到 OpenList 前端的 vite dev server (:3000)，
      // 由该 server 以 OPENLIST_PREVIEW_BASE="/openlist-ui/" 启动并改写 index.html。
      // 入口仍是后端驱动：/api/preview/openlist-ui → 302 → /openlist-ui/（下面 /api proxy 覆盖）
      '/openlist-ui/': {
        target: 'http://127.0.0.1:3000',
        changeOrigin: true,
      },
      '/play': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        // Vite 8 (rolldown) requires manualChunks to be a function, not an object.
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Group major vendor libs into a single chunk
            const vendorLibs = ['vue', 'vue-router', '@ionic/vue', '@ionic/vue-router']
            for (const lib of vendorLibs) {
              if (id.includes(lib)) return 'vendor'
            }
            return 'vendor' // fallback: all other node_modules
          }
        },
      },
    },
  },
})
