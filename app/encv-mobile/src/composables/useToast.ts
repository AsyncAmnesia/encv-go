import { createAnimation } from '@ionic/vue'
import { toastController } from '@ionic/vue'

interface ToastOptions {
  message: string
  duration?: number
  color?: string
}

const MAX_STACK = 5
const activeToasts: Array<{ id: number; element: HTMLElement }> = []
let toastIdCounter = 0

const enterAnim = (baseEl: HTMLElement) => {
  void baseEl
  return createAnimation()
    .addElement(baseEl)
    .duration(360)
    .easing('cubic-bezier(0.34, 1.56, 0.64, 1)')
    .fromTo('transform', 'translateY(-32px) scale(0.92)', 'translateY(0) scale(1)')
    .fromTo('opacity', '0', '1')
}

const leaveAnim = (baseEl: HTMLElement) => {
  return createAnimation()
    .addElement(baseEl)
    .duration(240)
    .easing('ease-in')
    .fromTo('opacity', '1', '0')
    .fromTo('transform', 'translateY(0) scale(1)', 'translateY(-16px) scale(0.96)')
}

export async function showToast(options: ToastOptions) {
  const {
    message,
    duration = 2400,
    color = 'primary',
  } = options

  const id = ++toastIdCounter

  const toast = await toastController.create({
    message,
    duration: 0,
    position: 'top',
    cssClass: `encv-toast encv-toast--${color}`,
    enterAnimation: enterAnim,
    leaveAnimation: leaveAnim,
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
      try { await toast.dismiss({ role: 'timeout' }) } catch {}
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
    if (removed?.element) {
      try { removed.element.remove() } catch {}
    }
  }

  const safeTop = getSafeTop()

  activeToasts.forEach((t, idx) => {
    const el = t.element
    if (!el) return
    const wrapper = el.querySelector('.toast-wrapper') as HTMLElement | null
    if (wrapper) {
      const offset = safeTop + idx * 8
      el.style.setProperty('--encv-toast-stack-offset', `${offset}px`)
    }
  })
}

function getSafeTop(): number {
  try {
    const root = document.documentElement
    const computed = getComputedStyle(root)
    const envTop = computed.getPropertyValue('--ion-safe-area-top')
    if (envTop && envTop !== '') return parseFloat(envTop) || 0
  } catch {}
  return 16
}
