import type { Plugin, Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'
import { execSync } from 'child_process'
import { createHandlers } from './handlers'
import { setMockSuffix, getMockSuffix } from './file-system'

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

function parseMockParams(url: string): { enabled: boolean; suffix: string } {
  try {
    const u = new URL(url)
    const mockParam = u.searchParams.get('__mock')
    const suffixParam = u.searchParams.get('__mock_suffix')

    if (mockParam === '0') return { enabled: false, suffix: getMockSuffix() }
    if (mockParam === '1' || mockParam !== null) {
      if (suffixParam) setMockSuffix(suffixParam)
      return { enabled: true, suffix: suffixParam || getMockSuffix() }
    }
  } catch {}

  return { enabled: isMockEnabled(), suffix: getMockSuffix() }
}

export function createMockPlugin(): Plugin {
  let handlers: Record<string, Connect.NextHandleFunction> = {}
  let mockActive = false

  return {
    name: 'encv-mock-api',
    configureServer(server) {
      ensureMockDataExists()
      handlers = createHandlers(server.config.base)

      server.middlewares.use((req, res, next) => {
        const url = req.url || ''
        const params = parseMockParams(url)

        if (!params.enabled) {
          next()
          return
        }

        mockActive = true

        for (const [pattern, handler] of Object.entries(handlers)) {
          if (url.startsWith(pattern) || url.startsWith(`${server.config.base}${pattern.slice(1)}`)) {
            handler(req, res, next)
            return
          }
        }

        next()
      })

      console.log('[MOCK] API mock middleware registered')
      console.log(`[MOCK] Default alist-encrypt suffix: "${getMockSuffix()}"`)
      console.log('[MOCK] Activate with: ?__mock=1&__mock_suffix=.ae')
      console.log('[MOCK] Disable with: ?__mock=0')
    },
  }
}
