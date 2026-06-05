import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'
import fs from 'node:fs'
import sirv from 'sirv'

// Resolve the openlist fork's public/dist for dev-mode serving.
// In dev, the SPA is served directly by Vite (no Go embed).
// In prod (APK), the dist is bundled into the plugin APK's assets/dist/.
const OPENLIST_DIST = path.resolve(__dirname, '../openlist/Hi-Sillot-OpenList/public/dist')
const OPENLIST_DIST_EXISTS = fs.existsSync(path.join(OPENLIST_DIST, 'index.html'))
// `openlist-ui` requests are proxied (path-rewrite) to this upstream.
// In dev, this is a locally `go run .` OpenList instance.
const OPENLIST_UPSTREAM = process.env.OPENLIST_UPSTREAM || 'http://127.0.0.1:5244'

/**
 * Vite plugin: openlist-ui-proxy
 *
 *  1. `/openlist-ui/api/*`        → proxy to `OPENLIST_UPSTREAM/api/*` (path rewrite)
 *  2. `/openlist-ui/*`            → serve static files from Hi-Sillot-OpenList/public/dist/
 *                                   with SPA fallback to index.html for non-asset paths
 *
 * Why path-rewrite instead of two sub-routes?
 *   - OpenList SPA's hardcoded axios baseURL is `/`, so a single namespace `/openlist-ui/`
 *     cleanly maps to `OpenList(5244)/` while letting Vite's other proxies (`/api`, `/openlist`)
 *     continue to route to encv-go unchanged.
 *
 * Why cors is not a concern:
 *   - OpenList's Cors.AllowOrigins defaults to ["*"] (internal/conf/config.go:222),
 *     so the SPA at localhost:8100 calling /openlist-ui/api/* → Vite → OpenList(5244)
 *     works without CORS preflight (proxy is server-side, browser sees same origin).
 */
function openlistUiProxy(): Plugin {
  return {
    name: 'openlist-ui-proxy',
    configureServer(server) {
      if (!OPENLIST_DIST_EXISTS) {
        // Don't crash dev: many devs will clone the fork later. Log once and skip the
        // static-serving half. The API proxy will still work if OpenList is up.
        server.config.logger.warn(
          `\n[openlist-ui] dist not found at ${OPENLIST_DIST}\n` +
          `[openlist-ui] → run \`git clone --branch dev https://github.com/Hi-Sillot/OpenList.git app/openlist/Hi-Sillot-OpenList\` and download the frontend dist (see app/openlist/README.md §4.4)\n` +
          `[openlist-ui] → SPA static serving is DISABLED; API proxy to ${OPENLIST_UPSTREAM} still works.\n`
        )
      }

      // 1. /openlist-ui/api/* → upstream /api/*   (registered FIRST so it wins over static)
      //    Vite's middleware DOES strip the prefix → req.url = /ping, not /openlist-ui/api/ping
      server.middlewares.use('/openlist-ui/api', async (req, res, next) => {
        try {
          // req.url inside this middleware is the path AFTER /openlist-ui/api
          // e.g. /openlist-ui/api/ping → req.url = /ping → upstream = /api/ping
          const path = req.url || '/'
          const target = `${OPENLIST_UPSTREAM}/api${path.startsWith('/') ? path : '/' + path}`
          const headers: Record<string, string> = {}
          for (const [k, v] of Object.entries(req.headers)) {
            if (typeof v === 'string') headers[k] = v
            else if (Array.isArray(v)) headers[k] = v.join(', ')
          }
          // Pass through the original host so OpenList can build absolute URLs
          headers['host'] = new URL(OPENLIST_UPSTREAM).host
          const upstream = await fetch(target, {
            method: req.method,
            headers,
            // @ts-expect-error - Node fetch accepts a stream body in 18+
            body: ['GET', 'HEAD'].includes(req.method || '') ? undefined : (req as any),
            // @ts-expect-error - duplex needed when forwarding body
            duplex: 'half',
          } as any)
          res.statusCode = upstream.status
          upstream.headers.forEach((v, k) => {
            // Don't forward encoding headers (Node will re-encode)
            if (['content-encoding', 'transfer-encoding', 'connection'].includes(k.toLowerCase())) return
            res.setHeader(k, v)
          })
          const buf = Buffer.from(await upstream.arrayBuffer())
          res.end(buf)
        } catch (e: any) {
          res.statusCode = 502
          res.setHeader('content-type', 'text/plain; charset=utf-8')
          res.end(
            `openlist-ui api proxy error: ${e?.message || e}\n\n` +
            `Is OpenList running at ${OPENLIST_UPSTREAM}?\n` +
            `Set OPENLIST_UPSTREAM env var to point elsewhere.\n`
          )
        }
      })

      // 2. /openlist-ui/* → static files from dist/  (registered SECOND)
      if (OPENLIST_DIST_EXISTS) {
        const serve = sirv(OPENLIST_DIST, {
          dev: true,
          single: true,         // SPA fallback for non-asset paths
          etag: true,
          dotfiles: false,
        })
        // Pre-read and rewrite index.html once at startup.
        // OpenList's built index.html references assets via absolute URLs
        // like src="/assets/xxx.js" — at /openlist-ui/ these resolve to
        // http://host:8100/assets/... (the main encv-mobile app), causing
        // a blank page (wrong JS bundle loaded or 404).
        let rewrittenIndexHtml = ''
        try {
          const raw = fs.readFileSync(path.join(OPENLIST_DIST, 'index.html'), 'utf-8')
          rewrittenIndexHtml = raw
            // HTML attribute patterns (static tags)
            .replace(/src="\/assets\//g, 'src="/openlist-ui/assets/')
            .replace(/href="\/assets\//g, 'href="/openlist-ui/assets/')
            .replace(/href="\/manifest\.json"/g, 'href="/openlist-ui/manifest.json"')
            .replace(/data-src="\/assets\//g, 'data-src="/openlist-ui/assets/')
            .replace(/src=(['"])\/assets\//g, 'src=$1/openlist-ui/assets/')
            .replace(/href=(['"])(?!\/openlist-ui\/)(\/[^'"]*\.(js|css|ico|png|svg|json|woff2?)["'])/g,
              'href=$1/openlist-ui$2')
            // JS string literals — the preloads block dynamically creates
            // <script>/<link> elements with absolute paths like
            //   "src":"/assets/index-xxx.js"
            // These must also be rewritten or the browser loads the wrong bundle.
            .replace(/":\"\/assets\//g, '":"/openlist-ui/assets/')
            // Inject base_path so the OpenList SPA knows it's served under
            // /openlist-ui/ and routes API calls through our proxy prefix.
            .replace(
              /base_path:\s*undefined/,
              'base_path: "/openlist-ui/"',
            )
          server.config.logger.info(
            `[openlist-ui] Rewrote ${raw.length} → ${rewrittenIndexHtml.length} bytes ` +
            `(index.html absolute paths → /openlist-ui/ prefixed)`,
          )
        } catch (e: any) {
          server.config.logger.warn(`[openlist-ui] Failed to rewrite index.html: ${e.message}`)
        }

        server.middlewares.use('/openlist-ui', (req, res, next) => {
          const orig = req.url || '/'
          const stripped = orig.replace(/^\/openlist-ui\/?/, '/') || '/'

          // Decide SPA fallback vs. real file.
          // - Path has no extension or doesn't resolve to an existing file in dist
          //   → it's an SPA route (/, /@login, /@home, /@s/xxx, /@login?redirect=…)
          //   → serve our pre-rewritten index.html (paths injected with /openlist-ui/ prefix)
          // - Path resolves to a real file in dist
          //   → let sirv serve it as-is
          //
          // Why the file check matters: sirv with `single: true` would otherwise
          // fall back to the ORIGINAL (unrewritten) index.html for SPA routes,
          // which uses absolute /assets/... paths that resolve to the wrong origin
          // and produce a blank page (404 or wrong JS bundle).
          const hasExt = /\.[a-z0-9]{1,5}$/i.test(stripped)
          const candidate = path.join(OPENLIST_DIST, stripped)
          const isRealFile = hasExt && fs.existsSync(candidate) && fs.statSync(candidate).isFile()

          if (rewrittenIndexHtml && !isRealFile) {
            res.setHeader('Content-Type', 'text/html; charset=utf-8')
            res.setHeader('Content-Length', Buffer.byteLength(rewrittenIndexHtml))
            res.end(rewrittenIndexHtml)
            return
          }

          req.url = stripped
          serve(req as any, res as any, next)
        })
      }
    },
  }
}

export default defineConfig({
  plugins: [vue(), openlistUiProxy()],
  server: {
    port: 8100,
    // Listen on all interfaces so OpenPreview / external previews can reach
    // the Vite dev server (default is localhost-only which is IPv6-only on
    // some sandboxes, breaking IPv4 / hostname access).
    host: '0.0.0.0',
    // ⚠️ 2026-06-05: Trae IDE's sandbox proxy (agent-tool-host :16000)
    //   rewrites the request's Origin header to match the upstream port
    //   (e.g. browser at :16000 → vite at :5173 with `Origin: http://localhost:5173`).
    //   Vite's default `cors: true` reflects the (rewritten) Origin back, so
    //   ESM `import()` in the browser sees `Access-Control-Allow-Origin:
    //   http://localhost:5173` — mismatched with the page origin — and
    //   `Failed to fetch dynamically imported module`. Fetch() works because
    //   it doesn't enforce CORS on same-origin GET; ESM loader does.
    //   Force '*' so dev resources load under the sandbox proxy.
    cors: {
      origin: '*',
      credentials: false,
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
      // hijacking `/openlist-ui/*` (the new Vite middleware below).
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
