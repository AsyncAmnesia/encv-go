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
 * 撤销 /openlist-spa/ subpath 路由改造：OpenList 应在原始环境 / 跑，
 * iframe / fetch 均直访 :5244，无需 Vite proxy。
 * 但保留 `__openlist-health` 中间件：Node 端探测 5244，绕过浏览器 CORS，
 * 让 OpenListWebView 的状态机有可靠的 health 探测通道。
 */

/**
 * 自定义中间件：显式健康检查端点
 * 解决 fetch('http://127.0.0.1:5244/...', { mode: 'cors' }) 在 502 时被浏览器 CORS 拦截，
 * 导致 res.status 变成 0（opaque），state 误判为 loading 的问题。
 *
 * 直接在 Node 端用 fetch 探测 5244，回 JSON 给浏览器，带 CORS 头 → 永远可读。
 * 同源访问（plugin-openlist vite :5174 fetch 自己 /__openlist-health）也工作。
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
    // 撤销 /openlist-spa/ proxy：iframe / fetch 都直访 :5244
    // （见文件头注释：subpath 改造不可靠，OpenList 应在原始环境 / 跑）
  },
})
