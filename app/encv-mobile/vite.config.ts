import { defineConfig, type Plugin } from 'vite'
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
//
// D14 决策（spec/unify-sandbox-preview-port）：沙箱 dev HMR 修复
//   - vite 默认 server.hmr.host = server.host = '0.0.0.0'，
//     浏览器无法连接（沙箱 dev 浏览器跑在外部 trae.cn 域名）
//   - 用 dynamicHmrHostPlugin 在 enforce:'pre' 阶段拦截 @vite/client，
//     替换 __HMR_HOSTNAME__ / __HMR_PORT__ / __HMR_PROTOCOL__ / __HMR_BASE__
//     为 auto-detected 外部 host + 网关端口 16666
// =============================================================================

/**
 * 沙箱 dev 动态 HMR host 修复（主 app 版本）
 *
 * 与 plugin-openlist/web 的同名插件逻辑一致，但端口默认 16666 (preview-gateway)。
 * HMR config 来源优先级：env (HMR_HOST / HMR_PROTOCOL / HMR_CLIENT_PORT) >
 *                       auto-detect (首次请求 Host 头) > fallback 'localhost'。
 *
 * 工作流程：
 *   1. 浏览器 GET http://<external-host>:16666/ → 网关 → :8100 vite
 *   2. vite 中间件检测 req.headers.host = '<external-host>:16666' → 存 detected
 *   3. vite 响应 HTML (含 <script src="/@vite/client">)
 *   4. 浏览器 GET /@vite/client → 网关 → :8100 vite
 *   5. transform @vite/client → 本插件 enforce:'pre' 替换占位符
 *   6. 浏览器收到 client.mjs，HMR client 连接 ws://<external-host>:16666/?token=...
 *   7. 网关 WS upgrade → 路由到 :8100，HMR 成功
 */
function dynamicHmrHostPlugin(): Plugin {
  const envHmrHost = process.env.HMR_HOST
  const envHmrProtocol = process.env.HMR_PROTOCOL as 'ws' | 'wss' | undefined
  const envHmrClientPort = process.env.HMR_CLIENT_PORT

  let detectedHost: string | null = null
  let detectedProtocol: 'ws' | 'wss' = 'ws'
  let hostSource: 'env' | 'detected' | 'pending' = 'pending'

  function resolveHost(reqHost?: string, referer?: string): { host: string; protocol: 'ws' | 'wss' } {
    if (envHmrHost) {
      hostSource = 'env'
      return { host: envHmrHost, protocol: envHmrProtocol || 'ws' }
    }
    if (reqHost) {
      hostSource = 'detected'
      const proto = referer?.startsWith('https://') ? 'wss' : 'ws'
      return { host: reqHost.split(':')[0], protocol: proto }
    }
    return { host: 'localhost', protocol: 'ws' }
  }

  return {
    name: 'dynamic-hmr-host',
    enforce: 'pre',
    configureServer(server) {
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
      const isViteClient =
        id.includes('vite/dist/client/client.mjs') ||
        id.includes('vite/client') ||
        id.endsWith('@vite/client') ||
        id.includes('/@vite/client')
      if (!isViteClient) return null

      const { host, protocol } = resolveHost(detectedHost ?? undefined, undefined)
      const port = envHmrClientPort ? Number(envHmrClientPort) : 16666
      const base = '/'

      let modified = code
      modified = modified.replace(/__HMR_HOSTNAME__/g, JSON.stringify(host))
      modified = modified.replace(/__HMR_PORT__/g, String(port))
      modified = modified.replace(/__HMR_PROTOCOL__/g, JSON.stringify(protocol))
      modified = modified.replace(/__HMR_BASE__/g, JSON.stringify(base))
      return { code: modified, map: null }
    },
  }
}

export default defineConfig({
  plugins: [vue(), dynamicHmrHostPlugin()],
  server: {
    // 统一入口改 :8100（由 preview-gateway :16666 接管对外暴露）
    port: 8100,
    // 监听所有接口（沙箱 IPv6/IPv4 兼容）
    host: '0.0.0.0',
    // ⚠️ 沙箱 dev 必须允许任意 Host 头：
    //   - vite 5+ 默认 server.allowedHosts 锁 localhost / 127.0.0.1
    //   - 外部 trae.cn 域名会被 vite 拒绝（403 "Blocked request"）
    //   - preview-gateway 透传 Host 头（changeOrigin: false），
    //     vite 看到的是原始外部域名
    //   - 设 true 允许所有 Host
    allowedHosts: true,
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
