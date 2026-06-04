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
    // 沙箱预览：HMR 走本地直连 5173 即可（不走 16000 代理，16000 不支持 WebSocket 升级）
    // 强制 HMR 跟 HTTP 同端口，避免 vite 把 WS 拆到 +1 端口（这样反向代理才能命中）
    hmr: { port: 8100, clientPort: 8100 },
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
      // hijacking `/openlist-ui/*` (which is proxied via encv-go dev_preview_proxy).
      '/openlist/': {
        target: 'http://127.0.0.1:2025',
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
