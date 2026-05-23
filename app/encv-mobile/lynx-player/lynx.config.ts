import { defineConfig } from '@lynx-js/rspeedy'
import { pluginReactLynx } from '@lynx-js/react-rsbuild-plugin'

export default defineConfig({
  plugins: [
    pluginReactLynx({
      defaultDisplayLinear: false,
    }),
  ],
  source: {
    entry: './src/player/index.tsx',
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
