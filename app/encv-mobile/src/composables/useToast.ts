import { toastController } from '@ionic/vue'

export async function showToast(options: {
  message: string
  duration?: number
  color?: string
  position?: 'top' | 'bottom' | 'middle'
}) {
  const toast = await toastController.create({
    message: options.message,
    duration: options.duration ?? 2000,
    color: options.color ?? 'dark',
    position: options.position ?? 'bottom',
    buttons: [
      {
        text: '✕',
        role: 'cancel',
      },
    ],
  })
  await toast.present()
  return toast
}

export function addDismissOnClick(toast: HTMLIonToastElement) {
  toast.addEventListener('click', () => {
    toast.dismiss(undefined, 'cancel')
  })
}

export async function createDismissibleToast(options: {
  message: string
  duration?: number
  color?: string
  position?: 'top' | 'bottom' | 'middle'
}) {
  const toast = await toastController.create({
    message: options.message,
    duration: options.duration ?? 2000,
    color: options.color ?? 'dark',
    position: options.position ?? 'bottom',
  })
  addDismissOnClick(toast)
  await toast.present()
  return toast
}
