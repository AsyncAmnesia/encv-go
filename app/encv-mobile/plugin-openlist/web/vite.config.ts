import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

/**
 * plugin-openlist/web Vite 配置
 *
 * 关键配置：`base: './'`
 *   生产模式：WebView 加载 `file:///android_asset/openlist/index.html`
 *   资源路径必须用相对路径 `./assets/...`（Vite 默认 `/assets/...` 会在 file:// 下 404）
 *
 * Proxy 设计：
 *   /openlist-spa/*  →  http://127.0.0.1:5244/*
 *   /__openlist-health → 自定义中间件（Node 直连 5244，返回 CORS-OK JSON）
 *
 * 沙箱浏览器模式下，OpenList 后端跑在 5244（由 scripts/dev-openlist.sh 启动）。
 * 我们的 Capacitor UI 通过 Vite 代理访问后端，前端无 CORS 问题（同源）。
 *
 * 生产模式（Android WebView 在 plugin-openlist Content() 内）：
 *   - OpenList 后端跑在 127.0.0.1:5244（同一设备）
 *   - iframe 直接访问 http://127.0.0.1:5244/（不走 Vite 代理）
 *   - 通过 import.meta.env.PROD 区分
 */

/**
 * 自定义中间件：显式健康检查端点
 * 解决 fetch('/openlist-spa/...', { mode: 'cors' }) 在 502 时被浏览器 CORS 拦截，
 * 导致 res.status 变成 0（opaque），state 误判为 loading 的问题。
 *
 * 直接在 Node 端用 fetch 探测 5244，回 JSON 给浏览器，带 CORS 头 → 永远可读。
 */
function openlistHealthPlugin(): Plugin {
  return {
    name: 'openlist-health',
    configureServer(server) {
      server.middlewares.use('/__openlist-health', async (req, res) => {
        const start = Date.now()
        res.setHeader('Content-Type', 'application/json; charset=utf-8')
        res.setHeader('Access-Control-Allow-Origin', '*')
        res.setHeader('Cache-Control', 'no-store')

        const target = 'http://127.0.0.1:5244/api/public/settings'
        const ac = new AbortController()
        const timer = setTimeout(() => ac.abort(), 3000)
        try {
          const r = await fetch(target, { signal: ac.signal })
          clearTimeout(timer)
          const elapsed = Date.now() - start
          res.statusCode = 200
          res.end(JSON.stringify({
            alive: true,
            upstreamStatus: r.status,
            latency: elapsed,
            target,
            ts: Date.now(),
          }))
        } catch (e: any) {
          clearTimeout(timer)
          const elapsed = Date.now() - start
          res.statusCode = 200
          res.end(JSON.stringify({
            alive: false,
            error: e?.name === 'AbortError' ? 'timeout' : (e?.message || String(e)),
            code: e?.cause?.code || e?.code,
            latency: elapsed,
            target,
            ts: Date.now(),
          }))
        }
      })
    },
  }
}

export default defineConfig({
  // ⚠️ 必须 './'：Android WebView 通过 file:///android_asset/openlist/ 加载
  // 绝对路径 '/assets/...' 在 file:// 协议下 404
  base: './',
  plugins: [vue(), openlistHealthPlugin()],
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
    // 生成包含 <base href="./"> 的 index.html（file:// 加载必需）
    // Vite 默认会处理，这里只是注释强调
  },
  server: {
    port: 5174,
    strictPort: false,
    proxy: {
      '/openlist-spa': {
        target: 'http://127.0.0.1:5244',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/openlist-spa/, ''),
        bypass: () => null,
      },
    },
  },
})
