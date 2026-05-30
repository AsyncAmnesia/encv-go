import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

const isDev = process.env.NODE_ENV !== 'production' || process.env.DEV === 'true'

let mockPlugin: any
if (isDev) {
  try {
    const { createMockPlugin } = require('./mock')
    mockPlugin = createMockPlugin()
  } catch (e) {
    console.warn('[vite] Mock plugin not available, skipping')
  }
}

export default defineConfig({
  plugins: [vue(), ...(mockPlugin ? [mockPlugin] : [])],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/health': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
      },
      '/stream': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/decrypt': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/preview': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/ping': {
        target: 'http://127.0.0.1:2026',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://127.0.0.1:2026',
        ws: true,
      },
    },
  },
})
