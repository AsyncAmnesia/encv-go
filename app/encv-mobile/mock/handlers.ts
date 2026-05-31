import type { Connect } from 'vite'
import * as http from 'http'
import * as fs from 'fs'
import * as path from 'path'

const WORKSPACE_ROOT = path.resolve(__dirname, '../../')
const MOCK_DATA_DIR = path.resolve(__dirname, '../__mock_data__')
const BACKEND = 'http://127.0.0.1:2025'

function json(res: Connect.ServerResponse, data: unknown, status = 200): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(data))
}

function loadJsonFile(filePath: string): Record<string, unknown> {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf-8'))
  } catch {
    return {}
  }
}

export function createHandlers(base: string): { dispatchRequest: Connect.NextHandleFunction } {
  const dispatchRequest: Connect.NextHandleFunction = (req, res) => {
    const url = new URL(req.url || '', `http://localhost${base}`)
    const pathname = url.pathname

    if (pathname === '/health') {
      return json(res, { status: 'ok' })
    }

    if (pathname === '/api/config') {
      return proxyAndRewriteMobileServerDir(req, res)
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}

function proxyAndRewriteMobileServerDir(req: Connect.IncomingMessage, res: Connect.ServerResponse): void {
  const devCfg = loadJsonFile(path.join(WORKSPACE_ROOT, 'config.dev.json'))
  const mobile = devCfg.mobile as Record<string, unknown> | undefined
  const targetDir = (mobile?.server_dir as string) || MOCK_DATA_DIR

  const proxyReq = http.request(BACKEND + req.url || '/api/config', {
    method: req.method,
    headers: { ...req.headers, host: '127.0.0.1:2025' },
  }, (proxyRes) => {
    const chunks: Buffer[] = []
    proxyRes.on('data', (chunk: Buffer) => chunks.push(chunk))
    proxyRes.on('end', () => {
      const body = Buffer.concat(chunks).toString('utf-8')
      try {
        const cfg = JSON.parse(body)
        if (cfg.mobile && typeof cfg.mobile.server_dir === 'string') {
          cfg.mobile.server_dir = targetDir
        }
        json(res, cfg, proxyRes.statusCode)
      } catch {
        res.writeHead(proxyRes.statusCode || 502, proxyRes.headers)
        res.end(body)
      }
    })
  })
  proxyReq.on('error', () => {
    res.writeHead(502, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'backend unreachable' }))
  })
  proxyReq.end()
}
