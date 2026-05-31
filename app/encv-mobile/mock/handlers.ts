import type { Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'
import {
  setMockSuffix,
  getMockSuffix,
  MOCK_PLUGINS,
  setMockFiles as fsSetMockFiles,
  addMockFile as fsAddMockFile,
  removeMockFile as fsRemoveFile,
  resetMockFiles,
  type MockFileItem,
} from './file-system'

const MOCK_DATA_ROOT = path.resolve(__dirname, '../__mock_data__')

function scanRealFiles(dirPath: string): MockFileItem[] {
  const results: MockFileItem[] = []
  if (!fs.existsSync(dirPath)) return results

  const entries = fs.readdirSync(dirPath, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dirPath, entry.name)
    const relPath = '/' + path.relative(MOCK_DATA_ROOT, fullPath).replace(/\\/g, '/')

    if (entry.isDirectory()) {
      if (entry.name === '.' || entry.name === '..') continue
      results.push({
        name: entry.name,
        path: relPath + '/',
        isDirectory: true,
        size: undefined,
      })
    } else if (entry.isFile()) {
      try {
        const stat = fs.statSync(fullPath)
        results.push({
          name: entry.name,
          path: relPath,
          isDirectory: false,
          size: stat.size,
          modified: stat.mtime.toISOString(),
        }) as MockFileItem
      } catch {}
    }
  }
  return results
}

const MIME_MAP: Record<string, string> = {
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.png': 'image/png',
  '.mp4': 'video/mp4',
  '.mkv': 'video/x-matroska',
  '.mp3': 'audio/mpeg',
  '.flac': 'audio/flac',
  '.pdf': 'application/pdf',
  '.txt': 'text/plain; charset=utf-8',
  '.csv': 'text/csv; charset=utf-8',
  '.ae': 'application/octet-stream',
  '.sccgv': 'application/octet-stream',
  '.scext': 'application/octet-stream',
  '.scepkg': 'application/octet-stream',
}

function readBody(req: Connect.IncomingMessage): Promise<string> {
  return new Promise((resolve) => {
    let body = ''
    req.on('data', (chunk) => { body += chunk })
    req.on('end', () => resolve(body))
  })
}

function resolveQueryPath(url: URL): string {
  return url.searchParams.get('path') || '/'
}

function resolveFiles(queryPath: string): MockFileItem[] {
  const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
  if (fs.existsSync(resolvedPath)) return scanRealFiles(resolvedPath)
  return []
}

function json(res: Connect.ServerResponse, data: unknown, status = 200): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(data))
}

let taskIdCounter = 1000
const tasks: Record<string, any> = {}

function fileSystemHandler(req: Connect.IncomingMessage, res: Connect.ServerResponse, base: string): boolean {
  const url = new URL(req.url || '', `http://localhost${base}`)
  const pathname = url.pathname

  if (pathname === '/api/files' || pathname.startsWith('/api/files?')) {
    const queryPath = resolveQueryPath(url)
    const tag = url.searchParams.get('tag')
    if (tag) return json(res, { files: [], error: 'tag filter not implemented in mock' })
    const files = resolveFiles(queryPath)
    return json(res, { files })
  }

  if (pathname === '/api/files/stream') {
    const queryPath = resolveQueryPath(url)
    const files = resolveFiles(queryPath)
    res.setHeader('Content-Type', 'text/event-stream')
    res.setHeader('Cache-Control', 'no-cache')
    res.flushHeaders()
    for (const f of files) res.write(`data: ${JSON.stringify(f)}\n\n`)
    res.write('data: [DONE]\n\n')
    res.end()
    return true
  }

  if (pathname === '/api/files/plugin-stream') {
    const queryPath = resolveQueryPath(url)
    const extParam = url.searchParams.get('extensions') || ''
    const extensions = extParam.split(',').map((e) => e.replace('.', '')).filter(Boolean)
    const allFiles = resolveFiles(queryPath)
    const filtered = allFiles.filter((f) => {
      if (f.isDirectory) return false
      const ext = f.name.includes('.') ? f.name.split('.').pop()?.toLowerCase() : ''
      return !!ext && extensions.includes(ext)
    })
    res.setHeader('Content-Type', 'text/event-stream')
    res.setHeader('Cache-Control', 'no-cache')
    res.flushHeaders()
    for (const f of filtered) res.write(`data: ${JSON.stringify(f)}\n\n`)
    res.write('data: [DONE]\n\n')
    res.end()
    return true
  }

  if (pathname === '/api/files/mkdir' && req.method === 'POST') {
    readBody(req).then(() => json(res, { ok: true }))
    return true
  }

  if (pathname === '/api/files/search') {
    const dirPath = url.searchParams.get('path') || '/'
    const keyword = (url.searchParams.get('keyword') || '').toLowerCase()
    const resolvedPath = path.join(MOCK_DATA_ROOT, dirPath.replace(/^\//, ''))
    if (!keyword || !fs.existsSync(resolvedPath)) return json(res, { files: [] })

    const results: MockFileItem[] = []
    function searchDir(dir: string): void {
      if (!fs.existsSync(dir)) return
      const entries = fs.readdirSync(dir, { withFileTypes: true })
      for (const entry of entries) {
        const full = path.join(dir, entry.name)
        const rel = '/' + path.relative(MOCK_DATA_ROOT, full).replace(/\\/g, '/')
        if (entry.name.toLowerCase().includes(keyword)) {
          results.push({ name: entry.name, path: rel, isDirectory: entry.isDirectory() } as MockFileItem)
        }
        if (entry.isDirectory()) searchDir(full)
      }
    }
    searchDir(resolvedPath)
    return json(res, { files: results })
  }

  if (pathname === '/api/files/exists') {
    const queryPath = resolveQueryPath(url)
    const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
    const exists = fs.existsSync(resolvedPath) && fs.statSync(resolvedPath).isFile()
    return json(res, { exists })
  }

  if (pathname === '/api/files/encrypt-output-exists') {
    return json(res, { exists: false, outputPath: '' })
  }

  if (pathname === '/api/files/tags') {
    if (req.method === 'GET') return json(res, { tags: [] })
    if (req.method === 'POST') return json(res, { ok: true })
    if (req.method === 'DELETE') return json(res, { ok: true })
    return false
  }

  if ((pathname === '/api/files' || pathname.startsWith('/api/files?')) && req.method === 'DELETE') {
    return json(res, { ok: true })
  }

  if ((pathname === '/api/files' || pathname.startsWith('/api/files?')) && (req.method === 'POST' || req.method === 'PATCH')) {
    readBody(req).then(() => json(res, { ok: true }))
    return true
  }

  return false
}

function fileContentHandler(req: Connect.IncomingMessage, res: Connect.ServerResponse, base: string): boolean {
  const url = new URL(req.url || '', `http://localhost${base}`)
  const pathname = url.pathname

  if (pathname === '/api/file' || pathname.startsWith('/api/file?')) {
    const queryPath = resolveQueryPath(url)

    if (req.method === 'GET') {
      const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
      if (!fs.existsSync(resolvedPath)) return json(res, { error: 'File not found' }, 404)
      try {
        const stat = fs.statSync(resolvedPath)
        const buf = fs.readFileSync(resolvedPath)
        const ext = path.extname(resolvedPath).toLowerCase()
        const textExts = ['.txt', '.csv', '.json', '.md', '.xml', '.html', '.css', '.js', '.ts', '.log']
        const encoding = textExts.includes(ext) ? 'utf-8' : 'binary'
        const content = textExts.includes(ext) ? buf.toString('utf-8') : buf.toString('base64')
        return json(res, { name: path.basename(resolvedPath), path: queryPath, size: stat.size, content, encoding })
      } catch (e: any) {
        return json(res, { error: e.message }, 500)
      }
    }

    if (req.method === 'POST' || req.method === 'PATCH') {
      readBody(req).then((body) => {
        try {
          const parsed = JSON.parse(body || '{}')
          if (req.method === 'PATCH' && parsed.path && parsed.new_name) {
            return json(res, { success: true, display_name: parsed.new_name })
          }
          if (parsed.oldPath && parsed.newName) return json(res, { ok: true })
          return json(res, { error: 'invalid request' }, 400)
        } catch {
          return json(res, { error: 'invalid json' }, 400)
        }
      })
      return true
    }

    if (req.method === 'DELETE') return json(res, { ok: true })

    return false
  }

  if (pathname === '/api/file/rename') {
    if (req.method === 'POST') {
      readBody(req).then(() => json(res, { ok: true }))
      return true
    }
    if (req.method === 'PATCH') {
      readBody(req).then((body) => {
        try {
          const parsed = JSON.parse(body || '{}')
          json(res, { success: true, display_name: parsed.new_name || '' })
        } catch { json(res, { error: 'invalid json' }, 400) }
      })
      return true
    }
    return false
  }

  if (pathname === '/api/file/copy' && req.method === 'POST') {
    readBody(req).then(() => json(res, { ok: true }))
    return true
  }

  if (pathname === '/api/file/move' && req.method === 'POST') {
    readBody(req).then(() => json(res, { ok: true }))
    return true
  }

  return false
}

function staticJsonHandler(pathname: string, _method: string, url: URL, res: Connect.ServerResponse): boolean {
  switch (pathname) {
    case '/health':
      return json(res, { status: 'ok' })
    case '/api/config': {
      const suffix = getMockSuffix()
      return json(res, {
        password: 'mock-password-for-testing',
        output_path: './output',
        server: { port: 2026, dir: '/' },
        plugin_settings: {
          video: { ext: '.sccgv', chunk_size_mb: 0, light_main_chunk_enabled: true, verify_after_pack: true, track_extensions: '.ass,.srt,.dm.ass,.vtt', skip_merge_for_split_mkv: false },
          image: { ext: '.sccgi' },
          audio: { ext: '.sccga' },
          text: { ext: '.sccgt' },
          wps: { ext: '.sccgwps' },
          pdf: { ext: '.sccgpdf' },
          alist_encrypt: { suffix },
        },
        mobile: {
          server_dir: '/storage/emulated/0',
          output_path: '/storage/emulated/0/encv-output',
        },
      })
    }
    case '/api/plugins':
      return json(res, { plugins: MOCK_PLUGINS })
    case '/api/permissions':
      return json(res, { storage: true })
    case '/api/container/versions':
      return json(res, {
        versions: [
          { version: 1, status: 'deprecated', label: 'v1 (legacy)' },
          { version: 2, status: 'stable', label: 'v2 (current)' },
          { version: 3, status: 'recommended', label: 'v3 (latest)' },
        ],
        default: 2,
      })
    case '/api/config/schema':
      return json(res, {})
    case '/api/index/stats':
      return json(res, {
        totalFiles: 30,
        totalDirs: 12,
        totalSize: 200000,
        indexedAt: new Date().toISOString(),
        isIndexing: false,
        lastBuildMs: 50,
      })
    case '/api/remote/info':
      return json(res, { webdav: { enabled: false, url: '', username: '', root: '' }, openlistSites: {} })
    case '/api/webdav/test-local':
      return json(res, { available: false, error: 'not configured in mock' })
    case '/api/webdav/test':
      return json(res, { success: false, reachable: false, error: 'mock mode' })
    case '/api/ffmpeg-status':
      return json(res, { ffmpeg_available: false, ffprobe_available: false, error: 'mock mode' })
    case '/api/build-info':
      return json(res, { app_version: '0.0.1-mock', ffmpeg_version: '', ffmpeg_codename: '' })
    case '/api/plugins/container-extensions':
      return json(res, { extensions: { video: '.sccgv', image: '.sccgi', audio: '.sccga', text: '.sccgt', pdf: '.sccgpdf' }, conflicts: [] })
    case '/api/file/text-preview-exts':
      return json(res, { extensions: ['txt', 'csv', 'json', 'md', 'xml', 'log', 'ini', 'yaml', 'yml', 'toml', 'env', 'sh', 'bat', 'ps1'], custom_extensions: [] })
    case '/api/alist-encrypt/decode-filename': {
      const encoded = url.searchParams.get('encoded') || ''
      if (encoded.endsWith('.ae')) {
        const name = encoded.replace(/\.ae$/, '').split('/').pop() || encoded
        return json(res, { plain_name: name, success: true })
      }
      return json(res, { plain_name: encoded, success: false })
    }
    case '/api/alist-encrypt/stream':
      res.setHeader('Content-Type', 'application/octet-stream')
      res.end(Buffer.alloc(0))
      return true
    case '/api/index/rebuild':
      return json(res, { ok: true, message: 'index rebuilt' })
    case '/api/index/clear':
      return json(res, { ok: true, message: 'index cleared' })
    default:
      if (pathname === '/api/webdav/test' && _method === 'POST') return json(res, { success: false, reachable: false, error: 'mock mode' })
      if (pathname === '/api/remote/openlist' && _method === 'POST') return json(res, { ok: true })
      if (pathname.startsWith('/api/remote/openlist/') && _method === 'PUT') return json(res, { ok: true })
      if (pathname.startsWith('/api/remote/openlist/') && _method === 'DELETE') return json(res, { ok: true })
      if (pathname === '/api/config' && _method === 'PUT') return json(res, { message: 'config updated', needsRestart: false })
      return false
  }
}

function taskMockHandler(req: Connect.IncomingMessage, res: Connect.ServerResponse, base: string): boolean {
  const url = new URL(req.url || '', `http://localhost${base}`)
  const pathname = url.pathname

  if (pathname === '/api/tasks/predict-plugin' && req.method === 'POST') {
    readBody(req).then((body) => {
      try {
        const parsed = JSON.parse(body || '{}')
        const sourcePath = parsed.sourcePath || ''
        const candidates: any[] = []

        for (const plugin of MOCK_PLUGINS) {
          if (sourcePath.endsWith(plugin.containerExtension)) {
            candidates.push({ name: plugin.name, matchType: 'container' as const, priority: 10, taskOptions: plugin.taskOptions })
          }
          for (const ext of plugin.supportedExtensions) {
            if (sourcePath.endsWith(`.${ext}`)) {
              candidates.push({ name: plugin.name, matchType: 'extension' as const, priority: 5, taskOptions: plugin.taskOptions })
            }
          }
        }

        const suffix = getMockSuffix()
        if (suffix && sourcePath.endsWith(suffix)) {
          candidates.push({
            name: 'alist-encrypt',
            matchType: 'extension' as const,
            priority: 8,
            taskOptions: { passwordStrategy: 'independent' as const, supportVersionSelect: false, supportedVersions: null, defaultVersion: 1, extraFields: [] },
          })
        }

        json(res, {
          candidates,
          pluginName: candidates.length > 0 ? candidates[0].name : null,
          taskOptions: candidates.length > 0 ? candidates[0].taskOptions : null,
        })
      } catch {
        json(res, { error: 'invalid request' }, 400)
      }
    })
    return true
  }

  if (pathname === '/api/tasks') {
    if (req.method === 'POST') {
      taskIdCounter++
      const task = { id: `mock-task-${taskIdCounter}`, type: 'encrypt', sourcePath: '/test.txt', status: 'queued' as const, progress: 0, createdAt: new Date().toISOString() }
      tasks[task.id] = task
      return json(res, task)
    }
    if (req.method === 'GET') return json(res, { tasks: Object.values(tasks) })
    return false
  }

  if (pathname.startsWith('/api/tasks/') && req.method === 'DELETE') {
    const id = pathname.split('/').pop() || ''
    if (tasks[id]) { delete tasks[id]; return json(res, { ok: true }) }
    return json(res, { error: 'task not found' }, 404)
  }

  if (pathname.startsWith('/api/tasks/') && req.method === 'POST') {
    const id = pathname.split('/').slice(-2)[0] || ''
    if (pathname.endsWith('/cancel')) return json(res, { ok: true })
    if (pathname.endsWith('/retry')) return json(res, { ok: true })
    return false
  }

  return false
}

function staticFileHandler(req: Connect.IncomingMessage, res: Connect.ServerResponse, base: string): boolean {
  const url = new URL(req.url || '', `http://localhost${base}`)
  const queryPath = resolveQueryPath(url)
  const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))

  if (!fs.existsSync(resolvedPath) || !fs.statSync(resolvedPath).isFile()) {
    res.statusCode = 404
    res.end('Not found')
    return true
  }

  const buf = fs.readFileSync(resolvedPath)
  const ext = path.extname(resolvedPath).toLowerCase()
  res.setHeader('Content-Type', MIME_MAP[ext] || 'application/octet-stream')
  res.setHeader('Content-Length', buf.length)
  res.setHeader('Accept-Ranges', 'bytes')
  res.end(buf)
  return true
}

function debugControlHandler(req: Connect.IncomingMessage, res: Connect.ServerResponse, base: string): boolean {
  const url = new URL(req.url || '', `http://localhost${base}`)
  const action = url.searchParams.get('action')

  try {
    if (action === 'set_suffix') {
      const suffix = url.searchParams.get('suffix') || '.ae'
      setMockSuffix(suffix)
      return json(res, { ok: true, suffix })
    }
    if (action === 'get_files') {
      const p = url.searchParams.get('path') || '/'
      return json(res, resolveFiles(p))
    }
    if (action === 'add_file') {
      const p = url.searchParams.get('path') || '/'
      const raw = url.searchParams.get('file')
      if (raw) { fsAddMockFile(p, JSON.parse(raw) as MockFileItem); return json(res, { ok: true }) }
      return json(res, { error: 'missing file param' }, 400)
    }
    if (action === 'remove_file') {
      const p = url.searchParams.get('path') || '/'
      const name = url.searchParams.get('name') || ''
      fsRemoveMockFile(p, name)
      return json(res, { ok: true })
    }
    if (action === 'reset') { resetMockFiles(); return json(res, { ok: true }) }
    if (action === 'status') return json(res, { enabled: true, suffix: getMockSuffix(), paths: Array.from(new Set(['/'])) })
    return json(res, { error: 'unknown action' }, 404)
  } catch (e: any) {
    return json(res, { error: e.message }, 500)
  }
}

export function createHandlers(base: string): { dispatchRequest: Connect.NextHandleFunction } {
  const dispatchRequest: Connect.NextHandleFunction = (req, res) => {
    const rawUrl = req.url || ''
    const url = new URL(rawUrl, `http://localhost${base}`)
    const pathname = url.pathname
    const method = req.method || 'GET'

    if (pathname === '/__mock_control') {
      debugControlHandler(req, res, base)
      return
    }

    if (pathname.startsWith('/api/files') && pathname !== '/api/file') {
      if (fileSystemHandler(req, res, base)) return
    }

    if (pathname === '/api/file' || pathname.startsWith('/api/file?') || pathname.startsWith('/api/file/')) {
      if (fileContentHandler(req, res, base)) return
    }

    if (pathname.startsWith('/api/tasks')) {
      if (taskMockHandler(req, res, base)) return
    }

    if (staticJsonHandler(pathname, method, url, res)) return

    if (pathname === '/stream' || pathname.startsWith('/stream?') || pathname === '/api/stream/external') {
      staticFileHandler(req, res, base)
      return
    }

    res.statusCode = 501
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ error: 'not implemented in mock', path: rawUrl }))
  }

  return { dispatchRequest }
}
