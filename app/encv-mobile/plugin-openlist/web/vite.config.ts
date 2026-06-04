import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

/**
 * plugin-openlist/web Vite 配置
 *
 * 关键配置：`base: './'` (生产) | `/openlist-ui/` (沙箱 dev, VITE_BASE env)
 *   生产模式：WebView 加载 `file:///android_asset/openlist/index.html`
 *   资源路径必须用相对路径 `./assets/...`（Vite 默认 `/assets/...` 会在 file:// 下 404）
 *
 * Proxy 设计（**重要：/openlist-spa 是 Hi-Sillot-OpenList-Frontend 的入口，不是 backend**）：
 *   /openlist-spa/*  →  http://127.0.0.1:3000/* (Hi-Sillot-OpenList-Frontend vite dev)
 *       plugin-openlist 的 OpenListWebView 通过 iframe 加载 /openlist-spa/#/login，
 *       iframe 内显示 Hi-Sillot-OpenList-Frontend (OpenListTeam 官方 web UI, SolidJS)
 *   /__openlist-health → 自定义中间件（Node 直连 5244，返回 CORS-OK JSON）
 *
 * 沙箱浏览器模式下，OpenList 后端跑在 5244（由 scripts/dev-openlist.sh 启动）。
 * 我们的 Capacitor UI 通过 Vite 代理访问 OpenList web UI，前端无 CORS 问题（同源）。
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

/**
 * 沙箱 dev / 真机 prod 区分
 *  - sandbox dev (VITE_BASE=/openlist-ui/): HTML base = /openlist-ui/
 *      原因：dev_preview_proxy 在 :2025 把 /openlist-ui/* 反代到本 vite :5174
 *      vite 收到 /openlist-ui/src/main.ts，base 匹配，serve web/src/main.ts
 *      资源路径是绝对 /openlist-ui/assets/...，浏览器解析为同源请求（:2025）
 *  - production (默认 './'): HTML base = ./
 *      原因：Android WebView 加载 file:///android_asset/openlist/index.html
 *      资源路径必须相对 ./assets/...（绝对路径在 file:// 协议下 404）
 */

export default defineConfig({
  // ⚠️ 沙箱 dev 用绝对 base（VITE_BASE），生产用相对 './'
  // 沙箱 dev：HTML 内 <base href="/openlist-ui/">，vite 处理 /openlist-ui/* 前缀
  // 生产：HTML 内 <base href="./">，Android WebView file:// 协议下加载相对资源
  base: process.env.VITE_BASE || './',
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
      // ⚠️ Hi-Sillot-OpenList-Frontend (OpenListTeam 官方 web UI) 的 vite dev server
      // 不是 OpenList backend：plugin-openlist iframe 通过 /openlist-spa/* 加载 OpenList web UI
      // OpenList web UI 自己有 vite proxy /api → :5244 backend（vite.config.ts 在 Hi-Sillot-OpenList-Frontend）
      '/openlist-spa': {
        target: 'http://127.0.0.1:3000',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/openlist-spa/, ''),
        bypass: () => null,
      },
    },
  },
})
