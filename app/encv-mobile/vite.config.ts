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
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
      },
      '/stream': {
        target: 'http://127.0.0.1:2025',
        changeOrigin: true,
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
