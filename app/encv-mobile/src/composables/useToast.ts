import { createAnimation, toastController } from '@ionic/vue'

interface ToastOptions {
  message: string
  duration?: number
  color?: string
}

const MAX_STACK = 5
const activeToasts: Array<{ id: number; element: HTMLElement }> = []
let toastIdCounter = 0

const enterAnimationBuilder = (baseEl: HTMLElement) => {
  return createAnimation()
    .addElement(baseEl.querySelector('.toast-wrapper') || baseEl)
    .duration(360)
    .easing('cubic-bezier(0.34, 1.56, 0.64, 1)')
    .fromTo('transform', 'translateY(-28px) scale(0.94)', 'translateY(0) scale(1)')
    .fromTo('opacity', '0', '1')
}

const leaveAnimationBuilder = (baseEl: HTMLElement) => {
  return createAnimation()
    .addElement(baseEl.querySelector('.toast-wrapper') || baseEl)
    .duration(220)
    .easing('ease-in')
    .fromTo('opacity', '1', '0')
    .fromTo('transform', 'translateY(0) scale(1)', 'translateY(-12px) scale(0.96)')
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
    enterAnimation: enterAnimationBuilder,
    leaveAnimation: leaveAnimationBuilder,
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
  applyStackOffset()

  if (duration > 0) {
    setTimeout(async () => {
      const idx = activeToasts.findIndex((t) => t.id === id)
      if (idx !== -1) {
        activeToasts.splice(idx, 1)
        applyStackOffset()
      }
      try { await toast.dismiss({ role: 'timeout' }) } catch {}
    }, duration)
  }

  toast.onDidDismiss().then(() => {
    const idx = activeToasts.findIndex((t) => t.id === id)
    if (idx !== -1) {
      activeToasts.splice(idx, 1)
      applyStackOffset()
    }
  })

  return toast
}

function applyStackOffset() {
  while (activeToasts.length > MAX_STACK) {
    const removed = activeToasts.shift()
    if (removed?.element) {
      try { removed.element.remove() } catch {}
    }
  }

  activeToasts.forEach((t, idx) => {
    const el = t.element
    if (!el) return
    const wrapper = el.querySelector('.toast-wrapper') as HTMLElement | null
    if (!wrapper) return
    const baseTop = getSafeTop()
    const offset = baseTop + idx * 8
    wrapper.style.setProperty('top', `${offset}px`, 'important')
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
