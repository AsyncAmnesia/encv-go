import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

/**
 * plugin-openlist/web Vite 配置
 *
 * Proxy 设计：
 *   /openlist-spa/*  →  http://127.0.0.1:5244/*
 *
 * 沙箱浏览器模式下，OpenList 后端跑在 5244（由 scripts/dev-openlist.sh 启动）。
 * 我们的 Capacitor UI 通过 Vite 代理访问后端，前端无 CORS 问题（同源）。
 *
 * 生产模式（Android WebView 在 plugin-openlist Content() 内）：
 *   - OpenList 后端跑在 127.0.0.1:5244（同一设备）
 *   - iframe 直接访问 http://127.0.0.1:5244/（不走 Vite 代理）
 *   - 通过 import.meta.env.PROD 区分
 */
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
  },
  server: {
    port: 5174,
    strictPort: false,
    proxy: {
      // 沙箱开发模式：把 /openlist-spa/* 反代到 OpenList 后端 5244
      // rewrite 把 /openlist-spa 前缀去掉（OpenList 后端不需要这个前缀）
      '/openlist-spa': {
        target: 'http://127.0.0.1:5244',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/openlist-spa/, ''),
        // 后端未启动时不抛错（让 iframe 显示 502/连接失败 UI）
        bypass: () => null,
      },
    },
  },
})
