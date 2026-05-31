import type { Connect } from 'vite'

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

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: pathname }))
  }

  return { dispatchRequest }
}
