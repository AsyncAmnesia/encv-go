import { defineConfig } from '@lynx-js/rspeedy'

export default defineConfig({
  source: {
    entry: './src/App.tsx',
  },
  output: {
    distPath: './dist',
  },
})
