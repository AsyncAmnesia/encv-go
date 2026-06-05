/**
 * preview-gateway
 * ===============
 *
 * Single-port reverse proxy for sandbox preview.
 *
 *   外网/浏览器 → :16000 (OpenPreview) → :16666 (gateway) → 4 upstream
 *   本地 dev    → :16666 (gateway) → 4 upstream
 *
 * D1 (用户决策："好记"): 监听 :16666
 *   - 避开 :16000 (agent-tool-host 占用)
 *   - 避开 :5173 (vite 老端口，由 preview-proxy 旧 default 占用)
 *   - 避开 :8100 (vite 新端口，本 spec 改的)
 *
 * Routes (see spec §3.1):
 *   /             → encv-mobile (Vite, :8100)         — default fallthrough
 *   /openlist-ui/ → plugin-openlist-web (Vite, :5174)
 *   /openlist/    → encv-go (Go, :2025)               — proxies to OpenList (:5244)
 *   /api          → encv-go (Go, :2025)
 *   /p            → encv-go (Go, :2025)
 *   /play         → encv-go (Go, :2025)
 *   /__gateway/health → gateway itself
 *
 * WebSocket:
 *   Upgrade on /             → ws://:8100 (main app HMR)
 *   Upgrade on /openlist-ui/ → ws://:5174 (plugin HMR)
 */

import http from 'node:http'
import https from 'node:https'
import { URL } from 'node:url'
import httpProxy from 'http-proxy'
import { WebSocketServer, type WebSocket } from 'ws'
import type { IncomingMessage, ClientRequest, ServerResponse } from 'node:http'
import type { Duplex } from 'node:stream'

// =============================================================================
// Config
// =============================================================================

const PORT = Number(process.env.PORT ?? 16666)
const HOST = process.env.HOST ?? '0.0.0.0'
const LOG_PREFIX = '[gateway]'

interface Upstream {
  /** URL prefix on the gateway, e.g. '/openlist-ui' */
  match: string
  /** HTTP target URL (no trailing slash) */
  target: string
  /** WebSocket target URL */
  wsTarget: string
  /** Human-readable name for logging */
  name: string
  /** Hint shown in 502 error */
  hint: string
  /**
   * Path rewrite function: transform req.url before forwarding to upstream.
   * Default = identity (path kept as-is). Use this to STRIP a path prefix
   * (e.g. '/openlist-ui/...' → '/...' for Vite, which serves from its own root).
   *
   * Why needed: Vite dev mode HTML outputs absolute paths like `/src/main.ts`
   * (no base prefix). Without pathRewrite, browser's follow-up requests go to
   * :16666/src/main.ts → fallthrough to :8100 (encv-mobile) → 404.
   * With strip prefix, :16666/openlist-ui/src/main.ts → :5174/src/main.ts → OK.
   */
  pathRewrite?: (path: string) => string
}

const UPSTREAMS: Upstream[] = [
  {
    match: '/openlist-ui',
    target: 'http://127.0.0.1:5174',
    wsTarget: 'ws://127.0.0.1:5174',
    name: 'plugin-openlist-web',
    hint: 'Check pm2 status for plugin-openlist-vite',
    // Strip /openlist-ui prefix: /openlist-ui/src/main.ts → /src/main.ts
    // （vite 收到 /src/main.ts，dev 模式下正常 serve）
    pathRewrite: (p) => p.replace(/^\/openlist-ui(?=\/|$)/, '') || '/',
  },
  {
    // /openlist 是 OpenList 真实前端入口（:5244），不是 encv-go 端点。
    // 原配置写的是 :2025（encv-go），但 encv-go 内部没有 `/openlist` 根路径
    // 的 reverse proxy（只注册了 /openlist/local/status、/openlist/sites 端点），
    // 直接转发到 :2025 会 404。
    // 正确做法：preview-gateway 自己把 /openlist/* 透传到 OpenList upstream :5244。
    // 需要 strip /openlist 前缀：/openlist → /、/openlist/xxx → /xxx
    //   （OpenList serve 在 :5244 根路径，不是 /openlist 命名空间）
    match: '/openlist',
    target: 'http://127.0.0.1:5244',
    wsTarget: 'ws://127.0.0.1:5244',
    name: 'openlist',
    hint: 'Check pm2 status for openlist (:5244)',
    pathRewrite: (p) => p.replace(/^\/openlist(?=\/|$)/, '') || '/',
  },
  {
    match: '/api',
    target: 'http://127.0.0.1:2025',
    wsTarget: 'ws://127.0.0.1:2025',
    name: 'encv-go',
    hint: 'Check pm2 status for start-preview (encv-go :2025)',
  },
  {
    match: '/p/',
    target: 'http://127.0.0.1:2025',
    wsTarget: 'ws://127.0.0.1:2025',
    name: 'encv-go',
    hint: 'Check pm2 status for start-preview (encv-go :2025)',
  },
  {
    match: '/play',
    target: 'http://127.0.0.1:2025',
    wsTarget: 'ws://127.0.0.1:2025',
    name: 'encv-go',
    hint: 'Check pm2 status for start-preview (encv-go :2025)',
  },
]

const DEFAULT_UPSTREAM: Upstream = {
  match: '/',
  target: 'http://127.0.0.1:8100',
  wsTarget: 'ws://127.0.0.1:8100',
  name: 'encv-mobile',
  hint: 'Check pm2 status for start-preview (encv-mobile vite :8100)',
}

const HEALTH_TIMEOUT_MS = 3000

// =============================================================================
// Logging
// =============================================================================

function log(...args: unknown[]): void {
  console.log(LOG_PREFIX, ...args)
}

function logUpstream(req: IncomingMessage, up: Upstream, status: 'OK' | 'FAIL', err?: unknown): void {
  const ip = req.socket.remoteAddress ?? '?'
  const ua = (req.headers['user-agent'] ?? '').slice(0, 50)
  if (err !== undefined) {
    log(`${ip} ${status} ${up.name} ${req.method} ${req.url} (${(err as Error).message ?? err}) ua="${ua}"`)
  } else {
    log(`${ip} ${status} ${up.name} ${req.method} ${req.url} ua="${ua}"`)
  }
}

// =============================================================================
// Route matching
// =============================================================================

/**
 * Match an incoming request to an upstream. Order matters — the first match wins.
 * - `/openlist-ui/...`  → plugin-openlist-web
 * - `/openlist/...`     → encv-go (proxies to OpenList)
 * - `/api...`           → encv-go
 * - `/p/...`            → encv-go
 * - `/play...`          → encv-go
 * - Referer contains `/openlist-ui/` (subresources of plugin SPA) → plugin-openlist-web
 * - Cookie `__plugin_spa=1` (set when user visits /openlist-ui/) → plugin-openlist-web
 *   (key for Vite dev mode: main.ts's absolute-root imports like /src/App.vue are
 *   dispatched to /openlist-ui subresources; without cookie they fallthrough to :8100
 *   and 404)
 * - default             → encv-mobile
 *
 * For path-prefix routes that are NOT followed by `/` (e.g. `/api`,
 * `/play`), we still match. The upstream's server is responsible for routing
 * the exact path within its namespace.
 */
function pickUpstream(url: string | undefined, referer: string | undefined, cookie: string | undefined): Upstream {
  if (!url) return DEFAULT_UPSTREAM
  for (const up of UPSTREAMS) {
    // 把 '/openlist-ui' 同时匹配 '/openlist-ui' 和 '/openlist-ui/...'
    // 把 '/openlist'   同时匹配 '/openlist'   和 '/openlist/...'
    // 把 '/api'        匹配 '/api'  和 '/api/...'
    if (url === up.match) return up
    if (url === up.match + '/') return up
    if (url.startsWith(up.match + '/')) return up
  }
  // Cookie-based fallback: when user has visited /openlist-ui/ in this session,
  // they've received a Set-Cookie: __plugin_spa=1. Subsequent subresource requests
  // (Vite's absolute-root paths like /src/App.vue) carry this cookie → route to :5174.
  if (cookie && /(?:^|;\s*)__plugin_spa=1/.test(cookie)) {
    return UPSTREAMS.find((u) => u.match === '/openlist-ui') ?? DEFAULT_UPSTREAM
  }
  // Referer-based fallback (works in some sandboxes; not in Trae IDE default policy).
  if (referer && /\/openlist-ui\//.test(referer)) {
    return UPSTREAMS.find((u) => u.match === '/openlist-ui') ?? DEFAULT_UPSTREAM
  }
  return DEFAULT_UPSTREAM
}

// =============================================================================
// HTTP proxy (per-upstream instance so error handlers are isolated)
// =============================================================================

function createProxyFor(up: Upstream): httpProxy {
  const proxy = httpProxy.createProxyServer({
    target: up.target,
    ws: false,           // ws handled separately via 'upgrade' event
    changeOrigin: false,  // CRITICAL: do NOT rewrite Origin/Host — see spec §3.3
    xfwd: true,           // add X-Forwarded-* headers (helps Vite detect proxy)
    preserveHeaderKeyCase: true,
    proxyTimeout: 30_000,
    timeout: 30_000,
  })

  // ⚠️ 沙箱 dev critical: override http-proxy's xfwd behavior for X-Forwarded-Proto.
  // http-proxy's built-in xfwd: true uses `req.socket.encrypted` to decide
  // X-Forwarded-Proto. If Trae proxy terminates TLS and forwards HTTP to us,
  // req.socket.encrypted=false → xfwd writes "http" → vite's @vite/client
  // injects `ws://...` → browser on https:// page gets SecurityError
  // "An insecure WebSocket connection may not be initiated from a page loaded over HTTPS".
  //
  // We override by reading `req.protocol` (Node IncomingMessage property) which
  // already honors the X-Forwarded-Proto header set by Trae proxy. If Trae didn't
  // set it, we fall back to 'https' for Trae-sandbox domains (match *.trae.cn or
  // has the well-known preview-gw prefix), otherwise 'http'.
  proxy.on('proxyReq', (proxyReq, req) => {
    const host = String(req.headers.host || '')
    const xfpRaw = req.headers['x-forwarded-proto']
    let xfpFirstStr: string = ''
    if (Array.isArray(xfpRaw) && xfpRaw.length > 0 && xfpRaw[0] !== undefined) {
      xfpFirstStr = xfpRaw[0] as string
    } else if (typeof xfpRaw === 'string') {
      xfpFirstStr = xfpRaw
    }
    const xfpFromIncoming = xfpFirstStr.toLowerCase().split(',')[0]?.trim() ?? ''
    let xfp = xfpFromIncoming
    if (!xfp) {
      // Heuristic: Trae sandbox external domains are HTTPS. The Trae proxy
      // terminates TLS at its edge; the connection from Trae to us is plain
      // HTTP, so req.protocol would say 'http' — but the user-facing URL is
      // HTTPS. Trust the host pattern.
      if (/trae\.cn$/i.test(host) || /agent-sandbox/i.test(host) || /^run-agent-/i.test(host)) {
        xfp = 'https'
      } else {
        const sock: any = req.socket
        if (sock?.encrypted) {
          xfp = 'https'
        } else {
          xfp = 'http'
        }
      }
    }
    if (xfp) {
      proxyReq.setHeader('X-Forwarded-Proto', xfp)
    }
  })

  // ⚠️ 沙箱 dev critical: when user visits /openlist-ui/ (plugin SPA entry),
  // inject Set-Cookie: __plugin_spa=1 so subsequent subresource requests
  // (Vite's absolute-root imports: /src/App.vue, /@fs/..., /node_modules/...)
  // can be routed to :5174 even when Referer is empty (Trae IDE default
  // referrer-policy strips it). This is the linchpin that makes
  // /openlist-ui/ not stay blank.
  if (up.match === '/openlist-ui') {
    proxy.on('proxyRes', (proxyRes, _req) => {
      // Only inject for HTML responses (the SPA entry document)
      const ct = proxyRes.headers['content-type']
      if (ct && /text\/html/i.test(String(ct))) {
        const existing = proxyRes.headers['set-cookie']
        const cookieLine = '__plugin_spa=1; Path=/; SameSite=Lax; Max-Age=3600'
        if (existing) {
          proxyRes.headers['set-cookie'] = Array.isArray(existing)
            ? [...existing, cookieLine]
            : [String(existing), cookieLine]
        } else {
          proxyRes.headers['set-cookie'] = [cookieLine]
        }
      }
    })
  }

  proxy.on('error', (err, req, resOrSocket) => {
    // Error can happen for both HTTP and WS requests
    if ('writeHead' in resOrSocket && typeof (resOrSocket as ServerResponse).writeHead === 'function') {
      const res = resOrSocket as ServerResponse
      // Only write 502 if headers not yet sent
      if (!res.headersSent) {
        res.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' })
        const body = {
          error: 'upstream_unavailable',
          upstream: up.name,
          target: up.target,
          path: req.url,
          hint: up.hint,
          detail: (err as Error).message ?? String(err),
        }
        res.end(JSON.stringify(body, null, 2))
      }
      logUpstream(req, up, 'FAIL', err)
    } else {
      // WebSocket or raw socket — destroy
      const sock = resOrSocket as Duplex
      sock.destroy()
      logUpstream(req, up, 'FAIL', err)
    }
  })

  return proxy
}

const proxies = new Map<string, httpProxy>()
for (const up of UPSTREAMS) proxies.set(up.name, createProxyFor(up))
proxies.set(DEFAULT_UPSTREAM.name, createProxyFor(DEFAULT_UPSTREAM))

// =============================================================================
// HTTP request handler
// =============================================================================

const server = http.createServer((req, res) => {
  // Health endpoint — handled inline, not proxied
  if (req.url === '/__gateway/health') {
    return handleHealth(req, res)
  }

  // Gateway-internal banner endpoint (handy for sanity check)
  if (req.url === '/__gateway') {
    res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' })
    res.end(JSON.stringify({
      name: 'preview-gateway',
      version: '1.0.0',
      see: '/__gateway/health',
    }, null, 2))
    return
  }

  const up = pickUpstream(req.url, req.headers.referer, req.headers.cookie)
  const proxy = proxies.get(up.name)
  if (!proxy) {
    res.writeHead(500, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'no_proxy_for_upstream', upstream: up.name }))
    return
  }

  // Apply pathRewrite (if any) before forwarding
  const originalUrl = req.url
  if (up.pathRewrite) {
    req.url = up.pathRewrite(req.url ?? '/')
  }

  // No body parsing — just forward the stream
  proxy.web(req, res, { target: up.target }, (err) => {
    // Restore req.url in case the connection is reused (defensive)
    if (up.pathRewrite) req.url = originalUrl
    // err already handled by proxy.on('error') listener; this callback
    // is only for sync errors during dispatch.
    if (!res.headersSent) {
      res.writeHead(500, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({
        error: 'gateway_dispatch_error',
        detail: (err as Error).message ?? String(err),
      }))
    }
  })
})

// =============================================================================
// WebSocket upgrade handler (HMR critical — see spec §3.4)
// =============================================================================

server.on('upgrade', (req, socket, head) => {
  const up = pickUpstream(req.url, req.headers.referer, req.headers.cookie)
  const proxy = proxies.get(up.name)
  if (!proxy) {
    socket.write('HTTP/1.1 500 Internal Server Error\r\n\r\n')
    socket.destroy()
    return
  }
  // Apply pathRewrite for WS too (HMR ws path: /openlist-ui/?token=... → /?token=...)
  if (up.pathRewrite) {
    req.url = up.pathRewrite(req.url ?? '/')
  }
  // http-proxy's `ws()` method handles the upgrade transparently
  proxy.ws(req, socket, head, { target: up.wsTarget }, (err) => {
    if (err) {
      logUpstream(req, up, 'FAIL', err)
      try {
        socket.write('HTTP/1.1 502 Bad Gateway\r\n\r\n')
        socket.destroy()
      } catch {
        // socket already destroyed
      }
    }
  })
})

// =============================================================================
// Health endpoint — concurrent ping of all upstreams (spec §3.5)
// =============================================================================

interface UpstreamHealth {
  url: string
  alive: boolean
  latency_ms: number
  error?: string
}

async function pingUpstream(up: Upstream): Promise<UpstreamHealth> {
  const start = Date.now()
  const url = new URL(up.target)
  const opts: https.RequestOptions = {
    hostname: url.hostname,
    port: url.port,
    path: '/',
    method: 'HEAD',
    timeout: HEALTH_TIMEOUT_MS,
    headers: { 'User-Agent': 'preview-gateway/health' },
  }
  return new Promise((resolve) => {
    const lib = url.protocol === 'https:' ? https : http
    const req = lib.request(opts, (res) => {
      res.resume() // drain
      resolve({
        url: up.target,
        alive: res.statusCode !== undefined && res.statusCode < 500,
        latency_ms: Date.now() - start,
      })
    })
    req.on('timeout', () => {
      req.destroy(new Error('timeout'))
    })
    req.on('error', (err) => {
      resolve({
        url: up.target,
        alive: false,
        latency_ms: Date.now() - start,
        error: (err as Error).message ?? String(err),
      })
    })
    req.end()
  })
}

async function handleHealth(_req: IncomingMessage, res: ServerResponse): Promise<void> {
  const all = [DEFAULT_UPSTREAM, ...UPSTREAMS]
  // Deduplicate by name (encv-go appears 4 times)
  const unique = new Map<string, Upstream>()
  for (const up of all) unique.set(up.name, up)
  const checks = await Promise.all(
    Array.from(unique.values()).map(async (up) => [up.name, await pingUpstream(up)] as const),
  )
  const upstreams: Record<string, UpstreamHealth> = {}
  for (const [name, h] of checks) upstreams[name] = h
  const ok = Object.values(upstreams).every((h) => h.alive)
  res.writeHead(ok ? 200 : 503, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify({ ok, upstreams }, null, 2))
}

// =============================================================================
// Startup
// =============================================================================

server.listen(PORT, HOST, () => {
  log(`listening on http://${HOST}:${PORT} (D1: 好记，16666)`)
  log(`routes:`)
  for (const up of [DEFAULT_UPSTREAM, ...UPSTREAMS]) {
    log(`  ${up.match.padEnd(20)} → ${up.target}  (${up.name})`)
  }
  log(`health:  http://${HOST}:${PORT}/__gateway/health`)
  log(`external: :16000 (OpenPreview) → :16666 (this gateway) after agent-browser navigate :16666 triggers auto-register`)
})

// Graceful shutdown (pm2 sends SIGINT)
for (const sig of ['SIGINT', 'SIGTERM'] as const) {
  process.on(sig, () => {
    log(`received ${sig}, closing...`)
    server.close(() => process.exit(0))
    setTimeout(() => process.exit(1), 5_000).unref()
  })
}
