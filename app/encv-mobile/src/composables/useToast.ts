import { toastController } from '@ionic/vue'

interface ToastOptions {
  message: string
  duration?: number
  color?: string
}

const MAX_STACK = 5
const activeToasts: Array<{ id: number; element: HTMLElement }> = []
let toastIdCounter = 0

export async function showToast(options: ToastOptions) {
  const {
    message,
    duration = 2500,
    color = 'primary',
  } = options

  const id = ++toastIdCounter

  const toast = await toastController.create({
    message,
    duration: 0,
    position: 'top',
    cssClass: `encv-toast encv-toast--${color}`,
    buttons: [
      {
        icon: 'close-outline',
        side: 'end',
        role: 'cancel',
      },
    ],
    animated: true,
    keyboardClose: false,
  })

  await toast.present()

  const toastEl = (toast as unknown as { el: HTMLElement }).el
  if (!toastEl) return

  activeToasts.push({ id, element: toastEl })
  repositionStack()

  if (duration > 0) {
    setTimeout(async () => {
      const idx = activeToasts.findIndex((t) => t.id === id)
      if (idx !== -1) {
        activeToasts.splice(idx, 1)
        repositionStack()
      }
      await toast.dismiss({ role: 'timeout' })
    }, duration)
  }

  toast.onDidDismiss().then(() => {
    const idx = activeToasts.findIndex((t) => t.id === id)
    if (idx !== -1) {
      activeToasts.splice(idx, 1)
      repositionStack()
    }
  })

  return toast
}

function repositionStack() {
  if (activeToasts.length === 0) return

  while (activeToasts.length > MAX_STACK) {
    const removed = activeToasts.shift()
    if (removed?.element) removed.element.remove()
  }

  const baseOffset = 12
  const gap = 8
  const safeTop = baseOffset + getSafeAreaInset()

  activeToasts.forEach((t, idx) => {
    const el = t.element
    if (!el) return
    const wrapper = el.querySelector('.toast-wrapper') as HTMLElement | null
    if (wrapper) {
      const offset = safeTop + idx * gap
      wrapper.style.transform = `translateY(${offset}px)`
      wrapper.style.transition = 'transform 0.3s cubic-bezier(0.25, 0.46, 0.45, 0.94)'
    }
  })
}

function getSafeAreaInset(): number {
  try {
    const root = document.documentElement
    const computed = getComputedStyle(root)
    const envTop = computed.getPropertyValue('--ion-safe-area-top')
    if (envTop && envTop !== '') return parseFloat(envTop) || 0
  } catch {}
  return 20
}
