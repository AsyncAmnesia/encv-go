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

    if (pathname === '/api/file' && req.method === 'GET') {
      const queryPath = url.searchParams.get('path') || ''
      if (queryPath.startsWith('/__mock_data__/') || queryPath.startsWith('__mock_data__/')) {
        const relativePath = queryPath.replace(/^\/?__mock_data__\//, '')
        const filePath = path.join(MOCK_DATA_ROOT, relativePath)

        if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
          const stat = fs.statSync(filePath)
          if (stat.size > 2 * 1024 * 1024) {
            return json(res, { error: 'file too large' }, 400)
          }
          const content = fs.readFileSync(filePath, 'utf-8')
          const encoding = Buffer.from(content, 'utf-8').toString('utf-8') === content ? 'utf-8' : 'binary'
          return json(res, {
            name: path.basename(filePath),
            path: queryPath,
            size: stat.size,
            content,
            encoding,
          })
        }
      }
      res.statusCode = 501
      res.end(JSON.stringify({ error: 'not implemented in mock for this path', path: queryPath }))
      return
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}
