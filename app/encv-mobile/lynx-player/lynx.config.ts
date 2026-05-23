import { defineConfig } from '@lynx-js/rspeedy'
import { pluginVueLynx } from 'vue-lynx/plugin'

export default defineConfig({
  plugins: [pluginVueLynx()],
  source: {
    entry: './src/main.ts',
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
