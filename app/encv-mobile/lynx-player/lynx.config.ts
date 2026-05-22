import { defineConfig } from '@lynx-js/rspeedy'
import { pluginReactLynx } from '@lynx-js/react-rsbuild-plugin'

export default defineConfig({
  plugins: [pluginReactLynx({ enableNewGesture: true })],
  source: {
    entry: './src/App.tsx',
  },
  output: {
    distPath: {
      root: './dist',
    },
    filename: {
      bundle: 'player.lynx.bundle',
    },
  },
})
