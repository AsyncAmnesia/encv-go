import { createApp } from 'vue-lynx'
import App from './App.vue'
import router from './router'

function getInitialRoute(): string {
  try {
    const lynxObj = (globalThis as any).lynx
    const globalProps = lynxObj?.__globalProps
    if (globalProps?.filePath) {
      return '/player'
    }
  } catch (_e) {
    // ignore
  }
  return '/'
}

const app = createApp(App)
app.use(router)
router.push(getInitialRoute())
app.mount()
