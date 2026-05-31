import type { Plugin, Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'
import { execSync } from 'child_process'
import { createHandlers } from './handlers'

const MOCK_DATA_ROOT = path.resolve(__dirname, '../__mock_data__')
const SCRIPT_PATH = path.resolve(__dirname, '../scripts/generate-mock-files.ts')

function isMockEnabled(): boolean {
  if (typeof window !== 'undefined') {
    return false
  }
  return true
}

function ensureMockDataExists(): void {
  if (!fs.existsSync(MOCK_DATA_ROOT)) {
    console.log('[MOCK] Generating mock data files...')
    try {
      execSync(`npx tsx "${SCRIPT_PATH}"`, {
        cwd: path.resolve(__dirname, '..'),
        stdio: 'pipe',
        timeout: 30000,
      })
      console.log(`[MOCK] ✅ Mock data generated at ${MOCK_DATA_ROOT}`)
    } catch (e: any) {
      console.warn(`[MOCK] ⚠️ Failed to generate mock data: ${e.message}`)
      console.warn('[MOCK] Run manually: npm run generate:mock')
    }
  }
}

function parseMockEnabled(url: string): boolean {
  try {
    const u = new URL(url)
    const mockParam = u.searchParams.get('__mock')
    if (mockParam === '0') return false
    if (mockParam === '1' || mockParam !== null) return true
  } catch {}
  return isMockEnabled()
}

const MOCK_API_PREFIXES = [
  '/health',
  '/decrypt',
]

function shouldMockIntercept(url: string): boolean {
  const pathname = url.split('?')[0]
  return MOCK_API_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(prefix))
}

export function createMockPlugin(): Plugin {
  let dispatchRequest: Connect.NextHandleFunction | null = null

  return {
    name: 'encv-mock-api',
    configureServer(server) {
      ensureMockDataExists()
      const { dispatchRequest: dispatcher } = createHandlers(server.config.base)
      dispatchRequest = dispatcher

      server.middlewares.use((req, res, next) => {
        const url = req.url || ''

        if (url.startsWith('/decrypt')) {
          console.error('[DECRYPT-REQ] method=' + req.method + ' url=' + url)
          console.error('[DECRYPT-REQ] headers=' + JSON.stringify(req.headers))
        }

        const enabled = parseMockEnabled(url)

        if (!enabled || !shouldMockIntercept(url)) {
          next()
          return
        }

        if (dispatchRequest) {
          try {
            dispatchRequest(req, res, next)
          } catch (e: any) {
            if (!res.headersSent) {
              res.statusCode = 500
              res.setHeader('Content-Type', 'application/json')
              res.end(JSON.stringify({ error: e.message || 'mock error' }))
            }
          }
          return
        }

        next()
      })

      console.log('[MOCK] API mock middleware registered')
      console.log('[MOCK] Activate with: ?__mock=1')
      console.log('[MOCK] Disable with: ?__mock=0')
    },
  }
}
