import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// 注意：原 openlistUiProxy() Vite middleware 已删除（架构改为后端驱动）。
// OpenList UI 静态服务由 encv-go (port 2025) 承担，见 server.proxy['/openlist-ui/']。
// 路径：app/encv-mobile/vite.config.ts → server.proxy → 127.0.0.1:2025/openlist-ui/*
// 后端：internal/server/openlist_ui_handler.go handleStatic
// 配置：config.user.json → preview.openlist_ui_dir
// 与生产路径一致：APK 内 gomobile 进程从 filesDir/openlist/dist/ 服务。

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
      // /openlist-ui/* — 透传到 encv-go 后端，由 encv-go 从 preview.openlist_ui_dir
      // 静态服务 Hi-Sillot-OpenList/public/dist/。后端负责 SPA fallback + index.html
      // 路径重写；Vite 不再持任何 UI 状态（生产路径一致：APK 内 gomobile 进程自己服务）。
      '/openlist-ui/': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      // /api/preview/openlist-ui — encv-go 返回 302 → /openlist-ui/
      // 由 /api proxy 自然覆盖
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
