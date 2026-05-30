import type { Plugin, Connect } from 'vite'
import { createHandlers } from './handlers'
import { setMockSuffix, getMockSuffix } from './file-system'

function isMockEnabled(): boolean {
  if (typeof window !== 'undefined') {
    return false
  }
  return true
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
