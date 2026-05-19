import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'com.encvgo.app',
  appName: 'ENCV-go',
  webDir: 'dist',
  server: {
    androidScheme: 'https',
  },
  hooks: {
    afterSync: async () => {
      const { execSync } = await import('child_process')
      console.log('Running encv-post-cap-sync...')
      execSync('node scripts/post-cap-sync.mjs', {
        cwd: import.meta.dirname,
        stdio: 'inherit',
      })
      console.log('encv-post-cap-sync completed')
    },
  },
}

export default config
