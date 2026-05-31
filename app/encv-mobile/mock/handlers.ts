import type { Connect } from 'vite'
import * as fs from 'fs'
import * as path from 'path'
import {
  getFiles,
  setMockSuffix,
  getMockSuffix,
  MOCK_PLUGINS,
  setMockFiles as fsSetMockFiles,
  addMockFile as fsAddMockFile,
  removeMockFile as fsRemoveMockFile,
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
        })
      } catch {}
    }
  }
  return results
}

let taskIdCounter = 1000

export function createHandlers(base: string): Record<string, Connect.NextHandleFunction> {
  return {
    '/api/files': async (req, res, next) => {
      const url = new URL(req.url || '', `http://localhost${base}`)
      const queryPath = url.searchParams.get('path') || '/'
      const tag = url.searchParams.get('tag')

      if (tag) {
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ files: [], error: 'tag filter not implemented in mock' }))
        return
      }

      const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
      let files: MockFileItem[]

      if (fs.existsSync(resolvedPath)) {
        files = scanRealFiles(resolvedPath)
      } else {
        files = getFiles(queryPath)
      }

      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ files }))
    },

    '/api/files/stream': async (req, res, next) => {
      const url = new URL(req.url || '', `http://localhost${base}`)
      const queryPath = url.searchParams.get('path') || '/'
      const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
      let files: MockFileItem[]

      if (fs.existsSync(resolvedPath)) {
        files = scanRealFiles(resolvedPath)
      } else {
        files = getFiles(queryPath)
      }

      res.setHeader('Content-Type', 'text/event-stream')
      res.setHeader('Cache-Control', 'no-cache')
      res.flushHeaders()

      for (const file of files) {
        res.write(`data: ${JSON.stringify(file)}\n\n`)
      }
      res.write('data: [DONE]\n\n')
      res.end()
    },

    '/api/files/plugin-stream': async (req, res, next) => {
      const url = new URL(req.url || '', `http://localhost${base}`)
      const queryPath = url.searchParams.get('path') || '/'
      const extParam = url.searchParams.get('extensions') || ''
      const extensions = extParam.split(',').map(e => e.replace('.', '')).filter(Boolean)

      const resolvedPath = path.join(MOCK_DATA_ROOT, queryPath.replace(/^\//, ''))
      let allFiles: MockFileItem[]

      if (fs.existsSync(resolvedPath)) {
        allFiles = scanRealFiles(resolvedPath)
      } else {
        allFiles = getFiles(queryPath)
      }

      const filtered = allFiles.filter(f => {
        if (f.isDirectory) return false
        const ext = f.name.includes('.') ? f.name.split('.').pop()?.toLowerCase() : ''
        return ext && extensions.includes(ext)
      })

      res.setHeader('Content-Type', 'text/event-stream')
      res.setHeader('Cache-Control', 'no-cache')
      res.flushHeaders()

      for (const file of filtered) {
        res.write(`data: ${JSON.stringify(file)}\n\n`)
      }
      res.write('data: [DONE]\n\n')
      res.end()
    },

    '/api/plugins': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ plugins: MOCK_PLUGINS }))
    },

    '/api/config': async (req, res, next) => {
      const suffix = getMockSuffix()
      const config = {
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
      }
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify(config))
    },

    '/api/tasks': async (req, res, next) => {
      if (req.method === 'POST') {
        taskIdCounter++
        const task = {
          id: `mock-task-${taskIdCounter}`,
          type: 'encrypt',
          sourcePath: '/test.txt',
          status: 'queued' as const,
          progress: 0,
          createdAt: new Date().toISOString(),
        }
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify(task))
        return
      }
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ tasks: [] }))
    },

    '/health': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ status: 'ok' }))
    },

    '/api/permissions': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ storage: true }))
    },

    '/api/files/tags': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ tags: [] }))
    },

    '/api/alist-encrypt/stream': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/octet-stream')
      res.end(Buffer.alloc(0))
    },

    '/api/container/versions': async (req, res, next) => {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({
        versions: [
          { version: 1, status: 'deprecated', label: 'v1 (legacy)' },
          { version: 2, status: 'stable', label: 'v2 (current)' },
          { version: 3, status: 'recommended', label: 'v3 (latest)' },
        ],
        default: 2,
      }))
    },

    '/api/tasks/predict-plugin': async (req, res, next) => {
      let body = ''
      req.on('data', chunk => body += chunk)
      req.on('end', () => {
        try {
          const parsed = JSON.parse(body || '{}')
          const sourcePath = parsed.sourcePath || ''
          const candidates = []

          for (const plugin of MOCK_PLUGINS) {
            if (sourcePath.endsWith(plugin.containerExtension)) {
              candidates.push({
                name: plugin.name,
                matchType: 'container' as const,
                priority: 10,
                taskOptions: plugin.taskOptions,
              })
            }
            for (const ext of plugin.supportedExtensions) {
              if (sourcePath.endsWith(`.${ext}`)) {
                candidates.push({
                  name: plugin.name,
                  matchType: 'extension' as const,
                  priority: 5,
                  taskOptions: plugin.taskOptions,
                })
              }
            }
          }

          const suffix = getMockSuffix()
          if (suffix && sourcePath.endsWith(suffix)) {
            candidates.push({
              name: 'alist-encrypt',
              matchType: 'extension' as const,
              priority: 8,
              taskOptions: {
                passwordStrategy: 'independent' as const,
                supportVersionSelect: false,
                supportedVersions: null,
                defaultVersion: 1,
                extraFields: [],
              },
            })
          }

          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify({
            candidates,
            pluginName: candidates.length > 0 ? candidates[0].name : null,
            taskOptions: candidates.length > 0 ? candidates[0].taskOptions : null,
          }))
        } catch {
          res.statusCode = 400
          res.end(JSON.stringify({ error: 'invalid request' }))
        }
      })
    },

    '/__mock_control': async (req, res, next) => {
      const url = new URL(req.url || '', `http://localhost${base}`)
      const action = url.searchParams.get('action')

      try {
        if (action === 'set_suffix') {
          const suffix = url.searchParams.get('suffix') || '.ae'
          setMockSuffix(suffix)
          res.end(JSON.stringify({ ok: true, suffix }))
        } else if (action === 'get_files') {
          const path = url.searchParams.get('path') || '/'
          res.end(JSON.stringify(getFiles(path)))
        } else if (action === 'add_file') {
          const path = url.searchParams.get('path') || '/'
          const raw = url.searchParams.get('file')
          if (raw) {
            const file = JSON.parse(raw) as MockFileItem
            fsAddMockFile(path, file)
            res.end(JSON.stringify({ ok: true }))
          } else {
            res.statusCode = 400
            res.end(JSON.stringify({ error: 'missing file param' }))
          }
        } else if (action === 'remove_file') {
          const path = url.searchParams.get('path') || '/'
          const name = url.searchParams.get('name') || ''
          fsRemoveMockFile(path, name)
          res.end(JSON.stringify({ ok: true }))
        } else if (action === 'reset') {
          resetMockFiles()
          res.end(JSON.stringify({ ok: true }))
        } else if (action === 'status') {
          res.end(JSON.stringify({
            enabled: true,
            suffix: getMockSuffix(),
            paths: Array.from(new Set(['/', ...Object.keys({})])),
          }))
        } else {
          res.statusCode = 404
          res.end(JSON.stringify({ error: 'unknown action' }))
        }
      } catch (e: any) {
        res.statusCode = 500
        res.end(JSON.stringify({ error: e.message }))
      }
    },
  }
}
