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
}

const UPSTREAMS: Upstream[] = [
  {
    match: '/openlist-ui',
    target: 'http://127.0.0.1:5174',
    wsTarget: 'ws://127.0.0.1:5174',
    name: 'plugin-openlist-web',
    hint: 'Check pm2 status for plugin-openlist-vite',
  },
  {
    match: '/openlist/',
    target: 'http://127.0.0.1:2025',
    wsTarget: 'ws://127.0.0.1:2025',
    name: 'encv-go',
    hint: 'Check pm2 status for start-preview (encv-go :2025)',
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
 * - default             → encv-mobile
 *
 * For path-prefix routes that are NOT followed by `/` (e.g. `/api`,
 * `/play`), we still match. The upstream's server is responsible for routing
 * the exact path within its namespace.
 */
function pickUpstream(url: string | undefined): Upstream {
  if (!url) return DEFAULT_UPSTREAM
  for (const up of UPSTREAMS) {
    if (url === up.match) return up
    if (url.startsWith(up.match)) return up
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

  const up = pickUpstream(req.url)
  const proxy = proxies.get(up.name)
  if (!proxy) {
    res.writeHead(500, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'no_proxy_for_upstream', upstream: up.name }))
    return
  }

  // No body parsing — just forward the stream
  proxy.web(req, res, { target: up.target }, (err) => {
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
  const up = pickUpstream(req.url)
  const proxy = proxies.get(up.name)
  if (!proxy) {
    socket.write('HTTP/1.1 500 Internal Server Error\r\n\r\n')
    socket.destroy()
    return
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
