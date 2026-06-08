import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: [
      { find: '@', replacement: path.resolve(__dirname, './src') },
      // TDesign chat 在 pnpm 严格模式下，@tdesign-vue-next/chat 的 module 字段
      // 走不通 Node.js 原生 resolver。测试环境用 stub 替代（保留类型信息）。
      {
        find: '@tdesign-vue-next/chat',
        replacement: path.resolve(__dirname, './src/engines/__tests__/__mocks__/tdesign-chat.mjs'),
      },
    ],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: [
      '__tests__/**/*.test.ts',
      'src/__tests__/**/*.test.ts',
      'src/composables/**/*.test.ts',
      'src/engines/**/*.test.ts',
      'src/views/**/*.test.ts',
    ],
  },
})
