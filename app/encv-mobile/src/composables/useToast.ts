import { toastController } from '@ionic/vue'

interface ToastOptions {
  message: string
  duration?: number
  color?: string
}

export async function showToast(options: ToastOptions) {
  const {
    message,
    duration = 2400,
    color = 'primary',
  } = options

  const toast = await toastController.create({
    message,
    duration,
    position: 'top',
    cssClass: `encv-toast encv-toast--${color}`,
    buttons: [
      {
        icon: 'close-outline',
        side: 'end',
        role: 'cancel',
      },
    ],
  })

  await toast.present()
}
