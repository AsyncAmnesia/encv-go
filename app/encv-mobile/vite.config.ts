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
  plugins: [
    vue(),
    // 强制 alias plugin：在 vite 8 (rolldown 1.0.3) 自身 alias 失效时兜底。
    // 对 `import 'X from "@encvgo/components"'` 直接改写为 import 真实路径。
    {
      name: 'encvgo-monorepo-alias',
      enforce: 'pre',
      resolveId(source, importer) {
        if (source === '@encvgo/components') {
          return path.resolve(__dirname, 'packages/components/src/index.ts')
        }
        // 子路径导入：@encvgo/components/* → packages/components/src/*
        if (source.startsWith('@encvgo/components/')) {
          const sub = source.slice('@encvgo/components/'.length)
          return path.resolve(__dirname, 'packages/components/src', sub)
        }
        // 兼容旧的别名（@ 仍由 vite alias 处理）
        return null
      },
    },
    // 关键（2026-06-04）：vite 8.0.16 + rolldown 1.0.3 在请求绝对路径 /packages/components/src/*.ts
    // 时，HTTP parser 报 HPE_HEADER_OVERFLOW 返回 431（即使 fs.allow 包含 /workspace）。
    // 与之对比，/index.html、/@vite/client、/src/main.ts 都正常 200。
    // 100% 复现的 vite 8 bug。
    // 兜底：写一个静态文件中间件，在 vite 自己处理之前拦截 /packages/* 路径，
    // 用 fs.readFileSync 读真实文件，构造 200 + Content-Type 响应。
    {
      name: 'encvgo-monorepo-static-serve',
      configureServer(server) {
        server.middlewares.use('/packages', (req, res, next) => {
          // 注意：vite/connect 挂中间件在 /packages 时，req.url 已经被去掉 /packages 前缀
          // 即 req.url === '/components/src/index.ts'，而不是 '/packages/components/src/index.ts'
          // 必须手动把 /packages 拼回去，再去掉前导 /，再 resolve 到 encv-mobile 根
          const urlPath = req.url.split('?')[0] // 去掉 query
          const relative = ('/packages' + urlPath).replace(/^\/+/, '')
          const realPath = path.resolve(__dirname, relative)
          // 安全检查：必须在 /workspace/app/encv-mobile/packages/ 下
          const allowedRoot = path.resolve(__dirname, 'packages') + path.sep
          if (!realPath.startsWith(allowedRoot) && realPath !== allowedRoot.slice(0, -1)) {
            res.statusCode = 403
            res.end('Forbidden: ' + realPath)
            return
          }
          try {
            const fs = require('node:fs')
            const content = fs.readFileSync(realPath)
            const ext = path.extname(realPath)
            const mimeTypes: Record<string, string> = {
              '.ts': 'application/typescript; charset=utf-8',
              '.vue': 'text/javascript; charset=utf-8',
              '.js': 'application/javascript; charset=utf-8',
              '.mjs': 'application/javascript; charset=utf-8',
              '.json': 'application/json; charset=utf-8',
            }
            res.setHeader('Content-Type', mimeTypes[ext] || 'application/octet-stream')
            res.setHeader('Cache-Control', 'no-cache')
            res.statusCode = 200
            res.end(content)
          } catch (e: any) {
            res.statusCode = 404
            res.end('Not Found: ' + e.message)
          }
        })
      },
    },
  ],
  server: {
    // 关键（2026-06-04）：vite 端口必须与 start-preview.sh 的探测默认值（5173）一致，
    // 否则 dev_preview_proxy 启动时设置的 ENCV_DEV_MOBILE_VITE_URL 跟实际 vite 端口
    // 不匹配 → 所有 /* 路径反代到 5173 拿到 502 → 浏览器 iframe 出现 "Network Error"。
    // 之前 vite.config.ts 写死 8100，start-preview.sh 探测 5173 被占回退到 5174，
    // 实际 vite 跑 8100 = 跟 env var 永远对不上 → 100% 502。
    port: 5173,
    // Listen on all interfaces so OpenPreview / external previews can reach
    // the Vite dev server (default is localhost-only which is IPv6-only on
    // some sandboxes, breaking IPv4 / hostname access).
    host: '0.0.0.0',
    // 沙箱预览：HMR 走本地直连 5173 即可（不走 16000 代理，16000 不支持 WebSocket 升级）
    // 强制 HMR 跟 HTTP 同端口，避免 vite 把 WS 拆到 +1 端口（这样反向代理才能命中）
    hmr: { port: 5173, clientPort: 5173 },
    // Allow reading up to /workspace so Vite can serve
    //   - /packages/components/src/*  (encv-mobile 自己的 monorepo 包，OpenListView.vue import)
    //   - /app/openlist/Hi-Sillot-OpenList-Frontend/dist/* (OpenList fork dist)
    // 之前 allow: [path.resolve(__dirname, '..')] 只允许 /workspace/app，
    // packages/components 被解析到 /workspace/app/packages/components（不存在）→ 431。
    // fs: {
    //   allow: [path.resolve(__dirname, '..')],
    // },
    fs: {
      allow: [path.resolve(__dirname, '../..')],
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
    // 关键（2026-06-04）：vite 8 (rolldown) 不再支持简单 string alias（'@x': '/abs'），
    // 必须用对象形式 { find, replacement }。实测两种写法都不生效（import 保持原样），
    // 推测 rolldown 1.0.3 把 @encvgo/components 当成 npm module 解析，fall through 到
    // web URL `/packages/components/src/index.ts`（vite fs.allow 之外 → 431/502）。
    // 替代方案：写一个 inline resolveAlias plugin，强制拦截 @encvgo/components 改写。
    alias: [
      { find: '@', replacement: path.resolve(__dirname, 'src') + '/' },
      { find: '@encvgo/components', replacement: path.resolve(__dirname, 'packages/components/src/index.ts') },
    ],
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
