import { Capacitor } from '@capacitor/core'

export async function copyToClipboard(text: string): Promise<boolean> {
  if (Capacitor.isNativePlatform()) {
    try {
      const { Clipboard } = await import(/* @vite-ignore */ '@capacitor/clipboard')
      await Clipboard.write({ string: text })
      return true
    } catch {}
  }

  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {}
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.cssText = 'position:fixed;opacity:0;left:-9999px;top:-9999px'
    textarea.setAttribute('readonly', '')
    document.body.appendChild(textarea)
    const range = document.createRange()
    range.selectNodeContents(textarea)
    const sel = window.getSelection()
    sel?.removeAllRanges()
    sel?.addRange(range)
    textarea.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  } catch {}

  return false
}
