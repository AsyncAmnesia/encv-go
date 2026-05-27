import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
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
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/health': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      '/stream': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/decrypt': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/preview': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
        timeout: 120_000,
      },
      '/ping': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://127.0.0.1:2025',
        ws: true,
      },
    },
  },
})
