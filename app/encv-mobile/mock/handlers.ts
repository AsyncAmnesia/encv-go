import type { Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'

const MOCK_DATA_ROOT = path.resolve(__dirname, '../__mock_data__')

function json(res: Connect.ServerResponse, data: unknown, status = 200): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(data))
}

export function createHandlers(base: string): { dispatchRequest: Connect.NextHandleFunction } {
  const dispatchRequest: Connect.NextHandleFunction = (req, res) => {
    const url = new URL(req.url || '', `http://localhost${base}`)
    const pathname = url.pathname

    if (pathname === '/health') {
      return json(res, { status: 'ok' })
    }

    if (pathname === '/api/config') {
      return json(res, {})
    }

    if (pathname === '/api/plugins') {
      const pluginsPath = path.join(MOCK_DATA_ROOT, 'plugins.json')
      if (fs.existsSync(pluginsPath)) {
        const plugins = JSON.parse(fs.readFileSync(pluginsPath, 'utf-8'))
        return json(res, { plugins })
      }
      return json(res, { plugins: [] })
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}
