import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import path from 'node:path'

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
/**
 * 把 <base href="..."> 注入到 <head> 最早位置
 * 解决 Vite dev 把 <script src="/@vite/client"> 注入到 <head> 顶部，
 * 早于手写 <base>，导致 base href 不生效的问题。
 *
 * 实现思路：在 index.html 写 <!--VITE-BASE-HREF-PLACEHOLDER--> 占位符
 * (放在 <head> 第一个子元素位置 — Vite 不会移位)
 * plugin 在 transformIndexHtml 钩子 (order: 'pre') 把占位符替换成 <base> 标签。
 * 之所以不用 'pre' 钩子直接 prepend <base>：Vite 8 的 order: 'pre' 在某些
 * 内置 plugin 之后才执行（如 htmlRewritePlugin），导致 @vite/client 仍抢先注入。
 */
function injectBaseHref(href: string): Plugin {
  const basePrefix = href.replace(/\/$/, '')
  return {
    name: 'inject-base-href',
    transformIndexHtml: {
      order: 'post',  // 必须 'post' —— 这样 Vite 已注入 @vite/client 后我们才能改其 src
      handler(html) {
        let result = html
        // 1. 替换占位符为 <base> 标签
        result = result.replace(
          '<!--VITE-BASE-HREF-PLACEHOLDER-->',
          `<base href="${href}" />`,
        )
        // 2. 改 @vite/client 路径为 base-prefixed
        //    Vite dev 自动注入 <script src="/@vite/client"> 是绝对根路径，
        //    不被 base 影响（base 之后注入）。我们手动改它，让网关路由正确。
        if (basePrefix && result.includes('<script type="module" src="/@vite/client">')) {
          result = result.replace(
            '<script type="module" src="/@vite/client">',
            `<script type="module" src="${basePrefix}/@vite/client">`,
          )
        }
        return result
      },
    },
  }
}

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
 * 沙箱 dev 动态 HMR host 修复
 *
 * 根因：vite 默认 server.hmr.host = server.host = '0.0.0.0'，
 * 但浏览器无法连接 0.0.0.0 / localhost:5174（沙箱 dev 浏览器跑在外部 trae.cn 域名，
 * localhost 指向浏览器自己的机器，非沙箱服务器）。
 *
 * 修复：在 enforce:'pre' 阶段拦截 @vite/client 模块，替换 __HMR_HOSTNAME__ /
 *       __HMR_PORT__ / __HMR_PROTOCOL__ / __HMR_BASE__ 占位符为
 *       从首次 HTTP 请求的 Host 头检测出的外部域名 + 网关端口 16666。
 *
 * 工作流程：
 *   1. 浏览器 GET http://<external-host>:16666/openlist-ui/
 *   2. 网关剥前缀 → :5174 vite 收到 GET /
 *   3. vite 中间件 (configureServer) 检测 req.headers.host = '<external-host>:16666'
 *      → 存到 detectedHost / detectedProtocol
 *   4. vite 响应 HTML (含 <script src="/openlist-ui/@vite/client">)
 *   5. 浏览器 GET /openlist-ui/@vite/client → 网关剥前缀 → :5174 vite
 *   6. vite transform @vite/client → 本插件 enforce:'pre' 替换占位符
 *   7. 浏览器收到 client.mjs，HMR client 连接 ws://<external-host>:16666/?token=...
 *   8. 网关 WS upgrade → 路由到 :5174，HMR 成功
 *
 * 兜底：如果 HMR_HOST env 已设置，用 env 值；否则 auto-detect。
 */
function dynamicHmrHostPlugin(): Plugin {
  const envHmrHost = process.env.HMR_HOST
  const envHmrProtocol = process.env.HMR_PROTOCOL as 'ws' | 'wss' | undefined
  const envHmrClientPort = process.env.HMR_CLIENT_PORT

  let detectedHost: string | null = null
  let detectedProtocol: 'ws' | 'wss' = 'ws'
  let hostSource: 'env' | 'detected' | 'pending' = 'pending'

  function resolveHost(reqHost?: string, referer?: string): { host: string; protocol: 'ws' | 'wss' } {
    // 优先用 env
    if (envHmrHost) {
      hostSource = 'env'
      return { host: envHmrHost, protocol: envHmrProtocol || 'ws' }
    }
    // 其次 auto-detect
    if (reqHost) {
      hostSource = 'detected'
      const proto = referer?.startsWith('https://') ? 'wss' : 'ws'
      return { host: reqHost.split(':')[0], protocol: proto }
    }
    // 最后 fallback
    return { host: 'localhost', protocol: 'ws' }
  }

  return {
    name: 'dynamic-hmr-host',
    enforce: 'pre',
    configureServer(server) {
      // 中间件：捕获首次请求的 Host 头
      server.middlewares.use((req, res, next) => {
        if (hostSource === 'pending' && req.headers.host) {
          const r = resolveHost(req.headers.host, req.headers.referer)
          detectedHost = r.host
          detectedProtocol = r.protocol
          console.log(
            `[dynamic-hmr-host] source=${hostSource} host=${detectedHost} protocol=${detectedProtocol}`,
          )
        }
        next()
      })
    },
    transform(code, id) {
      // 拦截 vite 的 client.mjs，替换 HMR config 占位符
      const isViteClient =
        id.includes('vite/dist/client/client.mjs') ||
        id.includes('vite/client') ||
        id.endsWith('@vite/client') ||
        id.includes('/@vite/client')
      if (!isViteClient) return null

      const { host, protocol } = resolveHost(
        detectedHost ?? undefined,
        undefined, // protocol 已从 middleware 检测过
      )
      const port = envHmrClientPort ? Number(envHmrClientPort) : 16666
      const base = '/'

      // vite 占位符是裸标识符（如 __HMR_PORT__），vite 内部 transform 后续
      // 会 escapeReplacement 替换。enforce:'pre' 保证我们先替换，
      // vite 内部再 replace 时找不到占位符就跳过。
      let modified = code
      modified = modified.replace(/__HMR_HOSTNAME__/g, JSON.stringify(host))
      modified = modified.replace(/__HMR_PORT__/g, String(port))
      modified = modified.replace(/__HMR_PROTOCOL__/g, JSON.stringify(protocol))
      modified = modified.replace(/__HMR_BASE__/g, JSON.stringify(base))
      // 不要改 __WS_TOKEN__：vite 生成的 token，gateway 透传给上游即可
      return { code: modified, map: null }
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
  plugins: [vue(), openlistHealthPlugin(), injectBaseHref(process.env.VITE_BASE || '/openlist-ui/'), dynamicHmrHostPlugin()],
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
    host: '0.0.0.0',
    strictPort: false,
    // ⚠️ 沙箱 dev 必须允许任意 Host 头：
    //   - vite 5+ 默认 server.allowedHosts 锁 localhost / 127.0.0.1
    //   - 外部 trae.cn 域名（如 run-agent-...trae.cn）会被 vite 拒绝（403）
    //   - preview-gateway 透传 Host 头（changeOrigin: false），
    //     所以 vite 看到的是原始外部域名，不是 localhost
    //   - 设 true 允许所有 Host，匹配 preview-gateway 反代场景
    allowedHosts: true,
    // ⚠️ 沙箱 dev 必须扩展 fs.allow：
    //   - vite 默认 fs.allow 只允许项目根目录 + 其祖先
    //   - main.ts 内 import "/@fs/workspace/app/encv-mobile/node_modules/..." 引用的是
    //     encv-mobile 主 app 的 node_modules（plugin-openlist/web 自己没装 @ionic/vue）
    //   - 不扩 allow 时 vite 返回 403/404/SPA fallback → 浏览器收到 text/html →
    //     ES module loader 拒绝执行 → main.ts 中断 → 空白
    fs: {
      allow: [
        path.resolve(__dirname),
        path.resolve(__dirname, '..', '..', '..'),  // encv-mobile root（包含 monorepo node_modules）
        path.resolve(__dirname, '..', '..', '..', 'node_modules'),
        path.resolve('/workspace/app/encv-mobile'),
        path.resolve('/workspace/app/encv-mobile/node_modules'),
        path.resolve('/workspace'),
      ],
    },
  },
})
