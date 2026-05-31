import type { Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'

const MOCK_DATA_DIR = path.resolve(__dirname, '../__mock_data__')

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

    if (pathname === '/decrypt') {
      const filePath = url.searchParams.get('file') || url.searchParams.get('path') || ''
      console.error('[MOCK-DECRYPT] raw filePath query param:', JSON.stringify(filePath))
      console.error('[MOCK-DECRYPT] full URL:', req.url)

      const absPath = path.join(MOCK_DATA_DIR, filePath)
      console.error('[MOCK-DECRYPT] resolved absPath:', absPath)

      if (!filePath || filePath.includes('..')) {
        return json(res, { error: 'invalid file path' }, 400)
      }

      if (!fs.existsSync(absPath)) {
        const parentDir = path.dirname(absPath)
        let siblings: string[] = []
        try { siblings = fs.readdirSync(parentDir) } catch {}
        console.error('[MOCK-DECRYPT] file NOT FOUND on disk')
        return json(res, {
          error: 'file not found',
          debug: { receivedFilePath: filePath, resolvedAbsPath: absPath, siblings },
        }, 404)
      }

      const content = fs.readFileSync(absPath)
      const ext = path.extname(filePath).toLowerCase()
      const contentType = ext === '.txt' ? 'text/plain; charset=utf-8'
        : ext === '.pdf' ? 'application/pdf'
        : 'application/octet-stream'

      res.statusCode = 200
      res.setHeader('Content-Type', contentType)
      res.setHeader('Content-Length', content.length)
      res.end(content)
      console.error('[MOCK-DECRYPT] served file, size=', content.length, 'type=', contentType)
      return
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}
