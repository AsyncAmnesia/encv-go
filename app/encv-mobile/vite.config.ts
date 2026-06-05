import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// =============================================================================
// ENCV Mobile Vite Config
// =============================================================================
// D9 决策（spec/unify-sandbox-preview-port §3.1）: vite 是纯净 SPA dev server，
// 不做任何反向代理。统一由 preview-gateway (:16666) 接管跨上游转发。
//
// 历史胶水（已撤销）:
//   - `cors: { origin: '*' }` —— 用于绕过 agent-tool-host 的 Origin 改写。
//     现在 :16666 网关用 changeOrigin: false 透传 Origin 头，Vite 默认
//     cors=true 会 reflect origin，与浏览器 Origin 匹配，CORS 天然通过。
//   - `server.proxy: { '/api', '/p', '/openlist/', '/play' }` —— 跨上游转发。
//     全部迁移到 preview-gateway :16666 的 UPSTREAMS 列表。
//   - `openlistUiProxy` plugin —— /openlist-ui 在主 app 内嵌时用的辅助中间件。
//     实际路由现在由 preview-gateway :16666/openlist-ui → plugin-openlist-web :5174
//     独立处理（plugin-openlist-web vite 自己用 VITE_BASE=/openlist-ui/ 处理前缀）。
// =============================================================================

export default defineConfig({
  plugins: [vue()],
  server: {
    // 统一入口改 :8100（由 preview-gateway :16666 接管对外暴露）
    port: 8100,
    // 监听所有接口（沙箱 IPv6/IPv4 兼容）
    host: '0.0.0.0',
    // Vite 默认 cors=true 会 reflect Origin —— 配合 preview-gateway changeOrigin:false，
    // 链路 :16666 → :8100 看到的 Origin=Host 匹配，CORS 天然通过
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
