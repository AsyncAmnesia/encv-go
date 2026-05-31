import type { Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'

const WORKSPACE_ROOT = path.resolve(__dirname, '../../')

function json(res: Connect.ServerResponse, data: unknown, status = 200): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(data))
}

function loadJsonFile(filePath: string): Record<string, unknown> {
  try {
    const raw = fs.readFileSync(filePath, 'utf-8')
    return JSON.parse(raw)
  } catch {
    return {}
  }
}

function deepMerge(base: Record<string, unknown>, overlay: Record<string, unknown>): Record<string, unknown> {
  const result = { ...base }
  for (const [k, ov] of Object.entries(overlay)) {
    if (ov == null) continue
    const bv = result[k]
    if (bv && typeof bv === 'object' && !Array.isArray(bv) && typeof ov === 'object' && !Array.isArray(ov)) {
      result[k] = deepMerge(bv as Record<string, unknown>, ov as Record<string, unknown>)
    } else {
      result[k] = ov
    }
  }
  return result
}

function loadMergedConfig(): Record<string, unknown> {
  const userCfg = loadJsonFile(path.join(WORKSPACE_ROOT, 'config.user.json'))
  const devCfg = loadJsonFile(path.join(WORKSPACE_ROOT, 'config.dev.json'))
  return deepMerge(userCfg, devCfg)
}

export function createHandlers(base: string): { dispatchRequest: Connect.NextHandleFunction } {
  let cachedConfig: Record<string, unknown> | null = null

  const dispatchRequest: Connect.NextHandleFunction = (req, res) => {
    const url = new URL(req.url || '', `http://localhost${base}`)
    const pathname = url.pathname

    if (pathname === '/health') {
      return json(res, { status: 'ok' })
    }

    if (pathname === '/api/config') {
      if (!cachedConfig) {
        cachedConfig = loadMergedConfig()
      }
      return json(res, cachedConfig)
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}
